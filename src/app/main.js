import '../core/node-shim.js'
import { createLibp2p } from 'libp2p'
import { tcp } from '@libp2p/tcp'
import { noise } from '@chainsafe/libp2p-noise'
import { yamux } from '@chainsafe/libp2p-yamux'
import { multiaddr } from '@multiformats/multiaddr'
import { loadOrCreatePrivateKey } from '../core/identity.js'
import {
  FILES_PUSH_PROTOCOL,
  OUTBOX_DIR,
  RECEIVED_DIR,
  listOutboxFiles,
  listReceivedFiles,
  readOutboxPayload,
  readStreamToBuffer,
  safeFileName,
  saveReceivedPayload
} from '../core/file-transfer.js'
import { readJsonBody, sendHtml, sendJson } from '../core/http-utils.js'
import { appPage } from './pages/app-page.js'
import { createServer } from 'node:http'
import { createSocket } from 'node:dgram'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { createReadStream } from 'node:fs'
import { join } from 'node:path'
import { networkInterfaces, hostname } from 'node:os'

const WEB_HOST = process.env.APP_WEB_HOST ?? '0.0.0.0'
const WEB_PORT = Number(process.env.APP_WEB_PORT ?? 3000)
const P2P_HOST = process.env.APP_P2P_HOST ?? '0.0.0.0'
const P2P_PORT = process.env.APP_P2P_PORT ?? '0'
const DISCOVERY_PORT = Number(process.env.APP_DISCOVERY_PORT ?? 50197)
const DISCOVERY_GROUP = process.env.APP_DISCOVERY_GROUP ?? '239.255.77.77'
const RECEIVER_TTL_MS = 12000
const BROADCAST_INTERVAL_MS = 2000

const state = {
  mode: 'sender',
  name: hostname(),
  receivers: new Map(),
  incoming: []
}

const getIPv4s = () => {
  const addresses = []

  for (const items of Object.values(networkInterfaces())) {
    for (const item of items ?? []) {
      if (item.family === 'IPv4' && !item.internal) {
        addresses.push(item.address)
      }
    }
  }

  return addresses.length === 0 ? ['127.0.0.1'] : addresses
}

const getTcpPort = (node) => {
  for (const addr of node.getMultiaddrs()) {
    const match = addr.toString().match(/\/tcp\/(\d+)/)
    if (match != null) {
      return match[1]
    }
  }

  throw new Error('Could not find libp2p TCP listen port')
}

const getAnnounceAddrs = (node) => {
  const port = getTcpPort(node)
  const peerId = node.peerId.toString()

  return getIPv4s().map((ip) => `/ip4/${ip}/tcp/${port}/p2p/${peerId}`)
}

const getWebUrls = () => {
  return getIPv4s().map((ip) => `http://${ip}:${WEB_PORT}`)
}

const pruneReceivers = () => {
  const now = Date.now()

  for (const [peerId, receiver] of state.receivers) {
    if (now - receiver.seenAt > RECEIVER_TTL_MS) {
      state.receivers.delete(peerId)
    }
  }
}

const receiverList = () => {
  pruneReceivers()

  return Array.from(state.receivers.values())
    .sort((a, b) => a.name.localeCompare(b.name))
}

const startDiscovery = (node) => {
  const socket = createSocket({ type: 'udp4', reuseAddr: true })

  socket.on('message', (message) => {
    try {
      const announcement = JSON.parse(message.toString('utf8'))
      if (announcement.app !== 'p2ptest-lan' || announcement.role !== 'receiver') {
        return
      }

      if (announcement.peerId === node.peerId.toString()) {
        return
      }

      const addr = announcement.addrs?.[0]
      if (typeof addr !== 'string') {
        return
      }

      state.receivers.set(announcement.peerId, {
        peerId: announcement.peerId,
        name: announcement.name || announcement.peerId,
        addr,
        addrs: announcement.addrs,
        seenAt: Date.now()
      })
    } catch {}
  })

  socket.bind(DISCOVERY_PORT, '0.0.0.0', () => {
    socket.setBroadcast(true)
    socket.setMulticastLoopback(true)
    try {
      socket.addMembership(DISCOVERY_GROUP)
    } catch (err) {
      console.warn(`Could not join discovery group ${DISCOVERY_GROUP}:`, err.message)
    }

    for (const ip of getIPv4s()) {
      if (ip === '127.0.0.1') {
        continue
      }

      try {
        socket.addMembership(DISCOVERY_GROUP, ip)
      } catch {}
    }
  })

  const timer = setInterval(() => {
    if (state.mode !== 'receiver') {
      return
    }

    const announcement = Buffer.from(JSON.stringify({
      app: 'p2ptest-lan',
      role: 'receiver',
      peerId: node.peerId.toString(),
      name: state.name,
      addrs: getAnnounceAddrs(node),
      webUrls: getWebUrls(),
      at: new Date().toISOString()
    }))

    socket.send(announcement, DISCOVERY_PORT, DISCOVERY_GROUP)
    socket.send(announcement, DISCOVERY_PORT, '255.255.255.255')
  }, BROADCAST_INTERVAL_MS)

  return { socket, timer }
}

const sendOutboxToReceiver = async (node, peerId) => {
  const receiver = state.receivers.get(peerId)
  if (receiver == null) {
    throw new Error('Receiver not found or no longer visible')
  }

  return sendOutboxToAddr(node, receiver.addr)
}

const sendOutboxToAddr = async (node, addr) => {
  const payload = await readOutboxPayload()
  const stream = await node.dialProtocol(multiaddr(addr), FILES_PUSH_PROTOCOL)
  stream.send(Buffer.from(JSON.stringify({
    ...payload,
    sender: {
      peerId: node.peerId.toString(),
      name: state.name
    }
  })))
  await stream.close()

  return payload.files.map((file) => ({ name: file.name }))
}

const serveReceivedFile = async (req, res, name) => {
  const fileName = safeFileName(name)
  const path = join(RECEIVED_DIR, fileName)

  await readFile(path)
  res.writeHead(200, {
    'content-type': 'application/octet-stream',
    'content-disposition': `attachment; filename="${encodeURIComponent(fileName)}"`
  })
  createReadStream(path).pipe(res)
}

const buildState = async (node) => {
  return {
    mode: state.mode,
    name: state.name,
    peerId: node.peerId.toString(),
    addrs: getAnnounceAddrs(node),
    webUrls: getWebUrls(),
    outbox: await listOutboxFiles(),
    received: await listReceivedFiles(),
    receivers: receiverList(),
    incoming: state.incoming.slice(0, 12)
  }
}

const startWebServer = (node) => {
  const server = createServer(async (req, res) => {
    try {
      const url = new URL(req.url, `http://${req.headers.host}`)

      if (req.method === 'GET' && url.pathname === '/') {
        sendHtml(res, appPage)
        return
      }

      if (req.method === 'GET' && url.pathname === '/api/state') {
        sendJson(res, 200, await buildState(node))
        return
      }

      if (req.method === 'POST' && url.pathname === '/api/mode') {
        const body = await readJsonBody(req)
        if (body.mode !== 'sender' && body.mode !== 'receiver') {
          throw new Error('Mode must be sender or receiver')
        }

        state.mode = body.mode
        if (typeof body.name === 'string' && body.name.trim() !== '') {
          state.name = body.name.trim()
        }

        sendJson(res, 200, await buildState(node))
        return
      }

      if (req.method === 'POST' && url.pathname === '/api/files') {
        const body = await readJsonBody(req)
        await mkdir(OUTBOX_DIR, { recursive: true })

        for (const file of body.files ?? []) {
          const name = safeFileName(file.name)
          await writeFile(join(OUTBOX_DIR, name), Buffer.from(file.data, 'base64'))
        }

        sendJson(res, 200, await buildState(node))
        return
      }

      if (req.method === 'DELETE' && url.pathname === '/api/files') {
        const name = safeFileName(url.searchParams.get('name') ?? '')
        await rm(join(OUTBOX_DIR, name), { force: true })
        sendJson(res, 200, await buildState(node))
        return
      }

      if (req.method === 'POST' && url.pathname === '/api/send') {
        const body = await readJsonBody(req)
        const files = body.addr != null
          ? await sendOutboxToAddr(node, body.addr)
          : await sendOutboxToReceiver(node, body.peerId)
        sendJson(res, 200, { files })
        return
      }

      if (req.method === 'GET' && url.pathname.startsWith('/received/')) {
        await serveReceivedFile(req, res, decodeURIComponent(url.pathname.slice('/received/'.length)))
        return
      }

      sendJson(res, 404, { error: 'Not found' })
    } catch (err) {
      sendJson(res, 500, { error: err.message })
    }
  })

  server.listen(WEB_PORT, WEB_HOST, () => {
    console.log(`Web UI: http://127.0.0.1:${WEB_PORT}`)
    for (const url of getWebUrls()) {
      console.log(`LAN Web UI: ${url}`)
    }
  })

  return server
}

const main = async () => {
  const { privateKey, keyPath, created } = await loadOrCreatePrivateKey()

  const node = await createLibp2p({
    privateKey,
    addresses: {
      listen: [`/ip4/${P2P_HOST}/tcp/${P2P_PORT}`]
    },
    transports: [tcp()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()]
  })

  node.handle(FILES_PUSH_PROTOCOL, async (stream) => {
    try {
      const bytes = await readStreamToBuffer(stream)
      const payload = JSON.parse(bytes.toString('utf8'))
      const saved = await saveReceivedPayload(payload)
      state.incoming.unshift({
        at: new Date().toISOString(),
        sender: payload.sender,
        files: saved
      })
      state.incoming = state.incoming.slice(0, 20)
      console.log(`received ${saved.length} file(s)`)
    } catch (err) {
      console.error('receiver push handler error:', err)
      try {
        stream.abort(err)
      } catch {}
    }
  })

  await node.start()

  console.log('Peer ID:', node.peerId.toString())
  console.log(`${created ? 'Created' : 'Loaded'} key: ${keyPath}`)
  for (const addr of getAnnounceAddrs(node)) {
    console.log('P2P address:', addr)
  }

  startDiscovery(node)
  startWebServer(node)
}

main().catch(console.error)

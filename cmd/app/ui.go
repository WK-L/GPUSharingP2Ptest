package main

const appPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>P2P File Transfer</title>
  <style>
    :root {
      color-scheme: light;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f5f7fb;
      color: #1d2430;
    }

    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; }
    main { width: min(1120px, calc(100vw - 32px)); margin: 0 auto; padding: 28px 0; }
    header { display: flex; justify-content: space-between; align-items: flex-start; gap: 20px; margin-bottom: 20px; }
    h1 { margin: 0 0 8px; font-size: 32px; line-height: 1.1; letter-spacing: 0; }
    h2 { margin: 0 0 14px; font-size: 18px; letter-spacing: 0; }
    p { margin: 0; color: #5c6676; line-height: 1.5; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    .panel { background: #fff; border: 1px solid #dce2ec; border-radius: 8px; padding: 18px; box-shadow: 0 10px 28px rgba(27, 42, 67, 0.06); }
    .wide { grid-column: 1 / -1; }
    .role { display: flex; gap: 8px; padding: 4px; border: 1px solid #dce2ec; border-radius: 8px; background: #fff; }
    button { min-height: 42px; border: 0; border-radius: 8px; padding: 0 14px; font: inherit; font-weight: 700; color: #243044; background: #edf1f7; cursor: pointer; }
    button.primary, button.active { color: #fff; background: #1f6feb; }
    button.success { color: #fff; background: #16833a; }
    input[type="file"], input[type="text"] { width: 100%; min-height: 44px; border: 1px solid #c8d0dc; border-radius: 8px; padding: 10px 12px; font: inherit; background: #fbfcfe; }
    input[type="file"] { border-style: dashed; }
    .row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
    .row > input[type="file"] { flex: 1 1 280px; }
    code { display: block; overflow-wrap: anywhere; padding: 10px; border-radius: 8px; background: #eef2f7; line-height: 1.45; color: #172033; }
    ul { list-style: none; margin: 0; padding: 0; }
    li { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 12px 0; border-top: 1px solid #e6ebf2; }
    li:first-child { border-top: 0; }
    .meta { color: #667285; font-size: 13px; overflow-wrap: anywhere; }
    .status { min-height: 24px; margin-top: 12px; color: #1f6feb; font-weight: 700; }
    .receiver { align-items: flex-start; }
    .receiver button { flex: 0 0 auto; }
    .badges { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
    .badge { display: inline-flex; align-items: center; min-height: 30px; border-radius: 8px; padding: 0 10px; background: #edf1f7; color: #243044; font-size: 13px; font-weight: 700; }
    .badge.ok { background: #e6f4ea; color: #146c2e; }
    .badge.warn { background: #fff4db; color: #855c00; }
    .address-actions { display: flex; gap: 10px; align-items: flex-start; margin-top: 12px; }
    .address-actions code { flex: 1 1 320px; }
    .hidden { display: none; }
    @media (max-width: 760px) {
      main { width: min(100vw - 24px, 1120px); padding: 20px 0; }
      header { display: block; }
      .role { margin-top: 14px; }
      .grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>P2P File Transfer</h1>
        <p>Run this same app on each computer, choose a role, then send files over LAN, relay, or DHT-discovered peers.</p>
      </div>
      <div class="role">
        <button id="senderMode">Sender</button>
        <button id="receiverMode">Receiver</button>
      </div>
    </header>

    <section class="panel wide">
      <h2>This Node</h2>
      <p id="nodeName"></p>
      <code id="nodeAddrs">Loading...</code>
      <div class="status" id="status"></div>
    </section>

    <section class="panel wide">
      <h2>Network</h2>
      <div class="badges" id="networkBadges"></div>
      <p class="meta" id="rendezvous"></p>
      <div class="address-actions">
        <code id="circuitAddr">No circuit address yet.</code>
        <button id="copyCircuit">Copy Circuit Address</button>
      </div>
    </section>

    <section class="grid">
      <section class="panel" id="senderPanel">
        <h2>Sender</h2>
        <div class="row">
          <input id="files" type="file" multiple>
          <button class="primary" id="upload">Add Files</button>
        </div>
        <h2 style="margin-top:18px">Outbox</h2>
        <ul id="outbox"></ul>
      </section>

      <section class="panel" id="receiversPanel">
        <h2>Receivers On LAN</h2>
        <div class="row" style="margin-bottom:14px">
          <input id="manualAddr" type="text" placeholder="/ip4/192.168.1.20/tcp/50000/p2p/...">
          <button class="success" id="sendManual">Send to Address</button>
        </div>
        <ul id="receivers"></ul>
      </section>

      <section class="panel" id="receiverPanel">
        <h2>Receiver</h2>
        <label for="displayName">Display name</label>
        <input id="displayName" type="text" placeholder="Kitchen laptop">
        <div class="status" id="receiverStatus"></div>
        <h2 style="margin-top:18px">Received Files</h2>
        <ul id="received"></ul>
      </section>

      <section class="panel">
        <h2>Incoming Log</h2>
        <ul id="incoming"></ul>
      </section>
    </section>
  </main>

  <script>
    const status = document.querySelector('#status')
    const nodeName = document.querySelector('#nodeName')
    const nodeAddrs = document.querySelector('#nodeAddrs')
    const networkBadges = document.querySelector('#networkBadges')
    const rendezvous = document.querySelector('#rendezvous')
    const circuitAddr = document.querySelector('#circuitAddr')
    const copyCircuit = document.querySelector('#copyCircuit')
    const outbox = document.querySelector('#outbox')
    const receivers = document.querySelector('#receivers')
    const received = document.querySelector('#received')
    const incoming = document.querySelector('#incoming')
    const filesInput = document.querySelector('#files')
    const displayName = document.querySelector('#displayName')
    const senderMode = document.querySelector('#senderMode')
    const receiverMode = document.querySelector('#receiverMode')
    const senderPanel = document.querySelector('#senderPanel')
    const receiversPanel = document.querySelector('#receiversPanel')
    const receiverPanel = document.querySelector('#receiverPanel')
    const manualAddr = document.querySelector('#manualAddr')
    let currentMode = 'sender'
    let bestCircuitAddr = ''

    const fileToBase64 = (file) => new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result).split(',')[1])
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(file)
    })

    const setMode = async (mode) => {
      const res = await fetch('/api/mode', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ mode, name: displayName.value.trim() })
      })
      const body = await res.json()
      if (!res.ok) {
        status.textContent = body.error || 'Could not change mode.'
        return
      }
      renderState(body)
    }

    const renderListMessage = (list, text) => {
      list.innerHTML = ''
      const item = document.createElement('li')
      const label = document.createElement('span')
      label.textContent = text
      item.append(label)
      list.append(item)
    }

    const renderState = (state) => {
      currentMode = state.mode
      senderMode.classList.toggle('active', state.mode === 'sender')
      receiverMode.classList.toggle('active', state.mode === 'receiver')
      senderPanel.classList.toggle('hidden', state.mode !== 'sender')
      receiversPanel.classList.toggle('hidden', state.mode !== 'sender')
      receiverPanel.classList.toggle('hidden', state.mode !== 'receiver')
      if (document.activeElement !== displayName) displayName.value = state.name || ''
      nodeName.textContent = state.mode === 'receiver'
        ? 'Receiver mode: visible through LAN discovery and DHT rendezvous.'
        : 'Sender mode: choose a receiver and push your outbox.'
      nodeAddrs.textContent = state.addrs.join('\n')
      renderNetwork(state.network)

      outbox.innerHTML = ''
      if (state.outbox.length === 0) {
        renderListMessage(outbox, 'No files in outbox.')
      } else {
        for (const name of state.outbox) {
          const item = document.createElement('li')
          const label = document.createElement('span')
          const remove = document.createElement('button')
          label.textContent = name
          remove.textContent = 'Remove'
          remove.onclick = async () => {
            await fetch('/api/files?name=' + encodeURIComponent(name), { method: 'DELETE' })
            await loadState()
          }
          item.append(label, remove)
          outbox.append(item)
        }
      }

      receivers.innerHTML = ''
      if (state.receivers.length === 0) {
        renderListMessage(receivers, 'No receivers found yet.')
      } else {
        for (const receiver of state.receivers) {
          const item = document.createElement('li')
          item.className = 'receiver'
          const info = document.createElement('div')
          const title = document.createElement('strong')
          const meta = document.createElement('div')
          const send = document.createElement('button')
          title.textContent = receiver.name || receiver.peerId
          meta.className = 'meta'
          meta.textContent = (receiver.source ? receiver.source + ' - ' : '') + receiver.addr
          send.className = 'success'
          send.textContent = 'Send Files'
          send.onclick = async () => {
            status.textContent = 'Sending to ' + title.textContent + '...'
            const res = await fetch('/api/send', {
              method: 'POST',
              headers: { 'content-type': 'application/json' },
              body: JSON.stringify({ peerId: receiver.peerId })
            })
            const body = await res.json()
            status.textContent = res.ok ? 'Sent ' + body.files.length + ' file(s).' : body.error || 'Send failed.'
          }
          info.append(title, meta)
          item.append(info, send)
          receivers.append(item)
        }
      }

      received.innerHTML = ''
      if (state.received.length === 0) {
        renderListMessage(received, 'No files received yet.')
      } else {
        for (const name of state.received) {
          const item = document.createElement('li')
          const link = document.createElement('a')
          link.textContent = name
          link.href = '/received/' + encodeURIComponent(name)
          link.download = name
          item.append(link)
          received.append(item)
        }
      }

      incoming.innerHTML = ''
      if (state.incoming.length === 0) {
        renderListMessage(incoming, 'No incoming transfers yet.')
      } else {
        for (const event of state.incoming) {
          const item = document.createElement('li')
          const label = document.createElement('span')
          const meta = document.createElement('span')
          label.textContent = event.files.map((file) => file.name).join(', ')
          meta.className = 'meta'
          meta.textContent = new Date(event.at).toLocaleTimeString()
          item.append(label, meta)
          incoming.append(item)
        }
      }
    }

    const addBadge = (text, good) => {
      const badge = document.createElement('span')
      badge.className = 'badge ' + (good ? 'ok' : 'warn')
      badge.textContent = text
      networkBadges.append(badge)
    }

    const renderNetwork = (network) => {
      networkBadges.innerHTML = ''
      addBadge(network.relayService ? 'Relay service on' : 'Relay service off', network.relayService)
      addBadge(network.relayConfigured ? 'Relay configured' : 'No relay configured', network.relayConfigured)
      addBadge(network.hasCircuitAddr ? 'Circuit address ready' : 'Waiting for circuit address', network.hasCircuitAddr)
      addBadge(network.dhtEnabled ? 'DHT ' + network.dhtMode : 'DHT off', network.dhtEnabled)
      addBadge('DHT peers ' + network.dhtPeers, network.dhtPeers > 0)
      addBadge('Connected peers ' + network.connectedPeers, network.connectedPeers > 0)
      rendezvous.textContent = 'Rendezvous: ' + network.rendezvous + ' | Static relays: ' + network.staticRelayCount + ' | Bootstrap peers: ' + network.bootstrapPeerCount

      bestCircuitAddr = network.circuitAddrs[0] || ''
      circuitAddr.textContent = bestCircuitAddr || 'No circuit address yet.'
      copyCircuit.disabled = bestCircuitAddr === ''
    }

    const loadState = async () => {
      const res = await fetch('/api/state')
      renderState(await res.json())
    }

    document.querySelector('#upload').onclick = async () => {
      const selected = Array.from(filesInput.files)
      if (selected.length === 0) {
        status.textContent = 'Choose at least one file first.'
        return
      }
      status.textContent = 'Adding files...'
      const files = []
      for (const file of selected) files.push({ name: file.name, data: await fileToBase64(file) })
      const res = await fetch('/api/files', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ files })
      })
      const body = await res.json()
      status.textContent = res.ok ? 'Outbox updated.' : body.error || 'Upload failed.'
      filesInput.value = ''
      await loadState()
    }

    document.querySelector('#sendManual').onclick = async () => {
      const addr = manualAddr.value.trim()
      if (addr === '') {
        status.textContent = 'Paste a receiver multiaddr first.'
        return
      }
      status.textContent = 'Sending...'
      const res = await fetch('/api/send', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ addr })
      })
      const body = await res.json()
      status.textContent = res.ok ? 'Sent ' + body.files.length + ' file(s).' : body.error || 'Send failed.'
    }

    copyCircuit.onclick = async () => {
      if (bestCircuitAddr === '') {
        status.textContent = 'No circuit address to copy yet.'
        return
      }
      await navigator.clipboard.writeText(bestCircuitAddr)
      status.textContent = 'Circuit address copied.'
    }

    senderMode.onclick = () => setMode('sender')
    receiverMode.onclick = () => setMode('receiver')
    displayName.onchange = () => {
      if (currentMode === 'receiver') setMode('receiver')
    }
    loadState().catch((err) => { status.textContent = err.message })
    setInterval(loadState, 2000)
  </script>
</body>
</html>`

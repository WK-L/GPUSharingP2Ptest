package main

const appPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>P2P Docker Deploy</title>
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
    header { margin-bottom: 20px; }
    h1 { margin: 0 0 8px; font-size: 32px; line-height: 1.1; }
    h2 { margin: 0 0 14px; font-size: 18px; }
    p { margin: 0; color: #5c6676; line-height: 1.5; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    .panel { background: #fff; border: 1px solid #dce2ec; border-radius: 8px; padding: 18px; box-shadow: 0 10px 28px rgba(27, 42, 67, 0.06); }
    .wide { grid-column: 1 / -1; }
    button { min-height: 42px; border: 0; border-radius: 8px; padding: 0 14px; font: inherit; font-weight: 700; color: #243044; background: #edf1f7; cursor: pointer; }
    button.primary { color: #fff; background: #1f6feb; }
    button.success { color: #fff; background: #16833a; }
    input[type="file"], input[type="text"] { width: 100%; min-height: 44px; border: 1px solid #c8d0dc; border-radius: 8px; padding: 10px 12px; font: inherit; background: #fbfcfe; }
    input[type="file"] { border-style: dashed; }
    .row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
    .row > input[type="file"] { flex: 1 1 280px; }
    code { display: block; overflow-wrap: anywhere; padding: 10px; border-radius: 8px; background: #eef2f7; line-height: 1.45; color: #172033; }
    ul { list-style: none; margin: 0; padding: 0; }
    li { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; padding: 12px 0; border-top: 1px solid #e6ebf2; }
    li:first-child { border-top: 0; }
    .meta { color: #667285; font-size: 13px; overflow-wrap: anywhere; }
    .status { min-height: 24px; margin-top: 12px; color: #1f6feb; font-weight: 700; }
    .badges { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
    .badge { display: inline-flex; align-items: center; min-height: 30px; border-radius: 8px; padding: 0 10px; background: #edf1f7; color: #243044; font-size: 13px; font-weight: 700; }
    .badge.ok { background: #e6f4ea; color: #146c2e; }
    .badge.warn { background: #fff4db; color: #855c00; }
    .address-actions { display: flex; gap: 10px; align-items: flex-start; margin-top: 12px; }
    .address-actions code { flex: 1 1 320px; }
    @media (max-width: 760px) {
      main { width: min(100vw - 24px, 1120px); padding: 20px 0; }
      .grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <h1>P2P Docker Deploy</h1>
      <p>Each machine runs the same node. Nodes discover each other over LAN, mDNS, relay, and DHT, then push deployment bundles over libp2p.</p>
    </header>

    <section class="panel wide">
      <h2>This Node</h2>
      <div class="row" style="margin-bottom:14px">
        <input id="displayName" type="text" placeholder="node display name">
        <button class="primary" id="saveNode">Save Name</button>
      </div>
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
      <section class="panel">
        <h2>Deployment Bundles</h2>
        <div class="row">
          <input id="files" type="file" multiple>
          <button class="primary" id="upload">Upload Bundles</button>
        </div>
        <ul id="bundles" style="margin-top:18px"></ul>
        <h2 style="margin-top:18px">Deploy Settings</h2>
        <div class="row" style="margin-top:14px">
          <input id="deployArchive" type="text" placeholder="bundle.tar.gz">
        </div>
        <div class="row" style="margin-top:10px">
          <input id="deployProject" type="text" placeholder="project name">
        </div>
        <div class="row" style="margin-top:10px">
          <input id="deployCompose" type="text" placeholder="docker-compose.yml">
        </div>
        <div class="row" style="margin-top:10px">
          <input id="deployToken" type="text" placeholder="deploy token if required">
        </div>
      </section>

      <section class="panel">
        <h2>Discovered Nodes</h2>
        <div class="row" style="margin-bottom:14px">
          <input id="manualAddr" type="text" placeholder="/ip4/192.168.1.20/tcp/50000/p2p/...">
          <button id="deployManual">Deploy To Address</button>
        </div>
        <ul id="peers"></ul>
      </section>

      <section class="panel wide">
        <h2>Deployment Activity</h2>
        <ul id="deployments"></ul>
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
    const bundles = document.querySelector('#bundles')
    const peers = document.querySelector('#peers')
    const deployments = document.querySelector('#deployments')
    const filesInput = document.querySelector('#files')
    const displayName = document.querySelector('#displayName')
    const manualAddr = document.querySelector('#manualAddr')
    const deployArchive = document.querySelector('#deployArchive')
    const deployProject = document.querySelector('#deployProject')
    const deployCompose = document.querySelector('#deployCompose')
    const deployToken = document.querySelector('#deployToken')
    let bestCircuitAddr = ''

    const fileToBase64 = (file) => new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result).split(',')[1])
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(file)
    })

    const renderListMessage = (list, text) => {
      list.innerHTML = ''
      const item = document.createElement('li')
      const label = document.createElement('span')
      label.textContent = text
      item.append(label)
      list.append(item)
    }

    const renderState = (state) => {
      if (document.activeElement !== displayName) displayName.value = state.name || ''
      if (document.activeElement !== deployCompose && deployCompose.value === '') deployCompose.value = 'docker-compose.yml'
      nodeName.textContent = 'This node can discover peers and, when enabled, receive remote Docker deployments.'
      nodeAddrs.textContent = state.addrs.join('\n')
      renderNetwork(state.network)

      bundles.innerHTML = ''
      if (state.bundles.length === 0) {
        renderListMessage(bundles, 'No deployment bundles uploaded.')
      } else {
        for (const name of state.bundles) {
          const item = document.createElement('li')
          const label = document.createElement('span')
          const remove = document.createElement('button')
          label.textContent = name
          remove.textContent = 'Remove'
          remove.onclick = async () => {
            await fetch('/api/bundles?name=' + encodeURIComponent(name), { method: 'DELETE' })
            await loadState()
          }
          item.append(label, remove)
          bundles.append(item)
        }
      }

      peers.innerHTML = ''
      if (state.peers.length === 0) {
        renderListMessage(peers, 'No peers discovered yet.')
      } else {
        for (const peer of state.peers) {
          const item = document.createElement('li')
          const info = document.createElement('div')
          const title = document.createElement('strong')
          const meta = document.createElement('div')
          const deploy = document.createElement('button')
          title.textContent = peer.name || peer.peerId
          meta.className = 'meta'
          meta.textContent = (peer.source ? peer.source + ' - ' : '') + peer.addr + (peer.deployEnabled ? ' - deploy enabled' : ' - deploy disabled')
          deploy.textContent = peer.deployEnabled ? 'Deploy Bundle' : 'Deploy Disabled'
          deploy.disabled = !peer.deployEnabled
          deploy.onclick = async () => {
            await deployToTarget({ peerId: peer.peerId, label: title.textContent })
          }
          info.append(title, meta)
          item.append(info, deploy)
          peers.append(item)
        }
      }

      deployments.innerHTML = ''
      if (state.deployments.length === 0) {
        renderListMessage(deployments, 'No deployments yet.')
      } else {
        for (const event of state.deployments) {
          const item = document.createElement('li')
          const wrap = document.createElement('div')
          const title = document.createElement('strong')
          const meta = document.createElement('div')
          const output = document.createElement('code')
          title.textContent = event.projectName + ' - ' + event.status
          meta.className = 'meta'
          meta.textContent = [event.archiveName, event.source?.name || event.source?.peerId || 'unknown source node', new Date(event.at).toLocaleString()].join(' | ')
          output.textContent = event.output || 'No command output.'
          wrap.append(title, meta, output)
          item.append(wrap)
          deployments.append(item)
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
      addBadge(network.dockerDeploy ? 'Docker deploy on' : 'Docker deploy off', network.dockerDeploy)
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

    document.querySelector('#saveNode').onclick = async () => {
      const res = await fetch('/api/node', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ name: displayName.value.trim() })
      })
      const body = await res.json()
      status.textContent = res.ok ? 'Node name updated.' : body.error || 'Update failed.'
      if (res.ok) renderState(body)
    }

    document.querySelector('#upload').onclick = async () => {
      const selected = Array.from(filesInput.files)
      if (selected.length === 0) {
        status.textContent = 'Choose at least one deployment bundle first.'
        return
      }
      status.textContent = 'Uploading bundles...'
      const files = []
      for (const file of selected) files.push({ name: file.name, data: await fileToBase64(file) })
      const res = await fetch('/api/bundles', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ files })
      })
      const body = await res.json()
      status.textContent = res.ok ? 'Bundle library updated.' : body.error || 'Upload failed.'
      filesInput.value = ''
      await loadState()
    }

    const deployToTarget = async ({ peerId = '', addr = '', label = 'target' }) => {
      const archiveName = deployArchive.value.trim()
      if (archiveName === '') {
        status.textContent = 'Fill in the deployment bundle file name first.'
        return
      }
      status.textContent = 'Deploying bundle to ' + label + '...'
      const res = await fetch('/api/deploy', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          peerId,
          addr,
          archiveName,
          projectName: deployProject.value.trim(),
          composeFile: deployCompose.value.trim(),
          token: deployToken.value.trim()
        })
      })
      const body = await res.json()
      status.textContent = res.ok
        ? 'Deployment completed for ' + (body.projectName || label) + '.'
        : (body.message || body.error || 'Deployment failed.')
      await loadState()
    }

    document.querySelector('#deployManual').onclick = async () => {
      const addr = manualAddr.value.trim()
      if (addr === '') {
        status.textContent = 'Paste a peer multiaddr first.'
        return
      }
      await deployToTarget({ addr, label: 'manual address' })
    }

    copyCircuit.onclick = async () => {
      if (bestCircuitAddr === '') {
        status.textContent = 'No circuit address to copy yet.'
        return
      }
      await navigator.clipboard.writeText(bestCircuitAddr)
      status.textContent = 'Circuit address copied.'
    }

    loadState().catch((err) => { status.textContent = err.message })
    setInterval(loadState, 2000)
  </script>
</body>
</html>`

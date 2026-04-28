package app

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
    code { display: block; overflow-wrap: anywhere; white-space: pre-wrap; padding: 10px; border-radius: 8px; background: #eef2f7; line-height: 1.45; color: #172033; }
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
    .bundle-choice { display: flex; gap: 10px; align-items: flex-start; flex: 1 1 auto; }
    .bundle-choice input[type="radio"] { margin-top: 4px; }
    .bundle-copy { flex: 1 1 auto; }
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
      <p>Each machine runs the same node. Peer types are relay, renter, and provider. Nodes discover each other over LAN, mDNS, relay, and DHT, then push deployment bundles over libp2p.</p>
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
          <input id="artifactPaths" type="text" placeholder="returned artifact paths, comma-separated">
        </div>
        <p class="meta" style="margin-top:10px">Select one bundle above. Compose file defaults to <code style="display:inline; padding:2px 6px">docker-compose.yml</code>. Artifact paths are relative to the bundle root, so do not include the project name prefix.</p>
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

      <section class="panel wide">
        <h2>Returned Artifacts</h2>
        <ul id="artifacts"></ul>
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
    const artifacts = document.querySelector('#artifacts')
    const filesInput = document.querySelector('#files')
    const displayName = document.querySelector('#displayName')
    const manualAddr = document.querySelector('#manualAddr')
    const artifactPaths = document.querySelector('#artifactPaths')
    let bestCircuitAddr = ''
    let selectedBundle = ''

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

    const asArray = (value) => Array.isArray(value) ? value : []

    const normalizeNetwork = (network) => ({
      relayService: Boolean(network?.relayService),
      relayConfigured: Boolean(network?.relayConfigured),
      hasCircuitAddr: Boolean(network?.hasCircuitAddr),
      dhtEnabled: Boolean(network?.dhtEnabled),
      dhtMode: network?.dhtMode || '',
      dockerDeploy: Boolean(network?.dockerDeploy),
      dhtPeers: Number(network?.dhtPeers || 0),
      connectedPeers: Number(network?.connectedPeers || 0),
      rendezvous: network?.rendezvous || '',
      staticRelayCount: Number(network?.staticRelayCount || 0),
      bootstrapPeerCount: Number(network?.bootstrapPeerCount || 0),
      circuitAddrs: asArray(network?.circuitAddrs)
    })

    const renderState = (state) => {
      const bundlesList = asArray(state?.bundles)
      const peersList = asArray(state?.peers)
      const deploymentsList = asArray(state?.deployments)
      const artifactsList = asArray(state?.artifacts)
      const addrsList = asArray(state?.addrs)
      const network = normalizeNetwork(state?.network)

      if (document.activeElement !== displayName) displayName.value = state?.name || ''
      nodeName.textContent = 'This node is a ' + (state?.peerType || 'renter') + ' peer and can discover other peers over P2P for deployment.'
      nodeAddrs.textContent = addrsList.join('\n')
      renderNetwork(network)

      if (selectedBundle !== '' && !bundlesList.includes(selectedBundle)) selectedBundle = ''
      if (selectedBundle === '' && bundlesList.length > 0) selectedBundle = bundlesList[0]

      bundles.innerHTML = ''
      if (bundlesList.length === 0) {
        renderListMessage(bundles, 'No deployment bundles uploaded.')
      } else {
        for (const name of bundlesList) {
          const item = document.createElement('li')
          const choice = document.createElement('label')
          const radio = document.createElement('input')
          const copy = document.createElement('div')
          const title = document.createElement('strong')
          const meta = document.createElement('div')
          const remove = document.createElement('button')
          choice.className = 'bundle-choice'
          copy.className = 'bundle-copy'
          meta.className = 'meta'
          radio.type = 'radio'
          radio.name = 'deployBundle'
          radio.checked = selectedBundle === name
          radio.onchange = () => {
            selectedBundle = name
          }
          title.textContent = name
          meta.textContent = selectedBundle === name ? 'Selected for deployment.' : 'Select this bundle for the next deploy.'
          remove.textContent = 'Remove'
          remove.onclick = async () => {
            await fetch('/api/bundles?name=' + encodeURIComponent(name), { method: 'DELETE' })
            await loadState()
          }
          copy.append(title, meta)
          choice.append(radio, copy)
          item.append(choice, remove)
          bundles.append(item)
        }
      }

      peers.innerHTML = ''
      if (peersList.length === 0) {
        renderListMessage(peers, 'No peers discovered yet.')
      } else {
        for (const peer of peersList) {
          const item = document.createElement('li')
          const info = document.createElement('div')
          const title = document.createElement('strong')
          const meta = document.createElement('div')
          const deploy = document.createElement('button')
          title.textContent = (peer.name || peer.peerId) + ' [' + (peer.peerType || 'renter') + ']'
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
      if (deploymentsList.length === 0) {
        renderListMessage(deployments, 'No deployments yet.')
      } else {
        for (const event of deploymentsList) {
          const eventArtifacts = asArray(event?.artifacts)
          const item = document.createElement('li')
          const wrap = document.createElement('div')
          const title = document.createElement('strong')
          const meta = document.createElement('div')
          const command = document.createElement('code')
          const output = document.createElement('code')
          const logs = document.createElement('code')
          const artifactList = document.createElement('div')
          title.textContent = event.projectName + ' - ' + event.status
          meta.className = 'meta'
          meta.textContent = [event.archiveName, event.source?.name || event.source?.peerId || 'unknown source node', new Date(event.at).toLocaleString()].join(' | ')
          command.textContent = event.command || 'No command recorded.'
          output.textContent = event.output || 'No command output.'
          logs.textContent = event.logs || 'No compose logs.'
          artifactList.className = 'meta'
          artifactList.textContent = eventArtifacts.length ? 'Artifacts: ' + eventArtifacts.join(', ') : 'Artifacts: none'
          // wrap.append(title, meta, command, output, logs, artifactList) // 印出docker command
          wrap.append(title, meta, output, logs, artifactList)
          item.append(wrap)
          deployments.append(item)
        }
      }

      artifacts.innerHTML = ''
      if (artifactsList.length === 0) {
        renderListMessage(artifacts, 'No returned artifacts yet.')
      } else {
        for (const path of artifactsList) {
          const item = document.createElement('li')
          const link = document.createElement('a')
          link.textContent = path
          link.href = '/artifacts/' + path.split('/').map(encodeURIComponent).join('/')
          link.download = path.split('/').at(-1)
          item.append(link)
          artifacts.append(item)
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
      const archiveName = selectedBundle.trim()
      if (archiveName === '') {
        status.textContent = 'Select one deployment bundle first.'
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
          projectName: '',
          composeFile: '',
          artifactPaths: artifactPaths.value.trim()
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
    setInterval(loadState, 5000)
  </script>
</body>
</html>`

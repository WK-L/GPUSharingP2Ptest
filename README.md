# LAN P2P Transfer

A small libp2p LAN file-transfer app. Each computer runs the same app, picks a role in the browser, and transfers files directly over the local network.

## Install

Prerequisites:

- Node.js 20 or newer.
- npm, usually installed together with Node.js.
- Two computers on the same LAN if you want to test an actual peer-to-peer transfer.

Clone the project and install dependencies:

```bash
npm install
```

The app uses these local folders for file transfer data:

- `outbox/` - put sender files here if you want to prepare files manually.
- `received/` - receiver mode writes incoming files here.

If the folders do not exist yet, create them before running the app:

```bash
mkdir -p outbox received
```

## Run

```bash
npm run app
```

Open the local URL printed in the terminal. On another computer in the same LAN, open the printed LAN URL.

## Main Structure

- `src/app/` - the single web app for sender/receiver LAN transfers.
- `src/app/pages/` - HTML template used by the web server.
- `src/core/` - shared identity, file transfer, HTTP, and Node compatibility helpers.
- `outbox/` - local files queued by the sender UI.
- `received/` - files received by receiver mode.

## Scripts

- `npm run app` - start the single LAN sender/receiver web app.

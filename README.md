# P2P Docker Deploy

這個專案是一個 Go/libp2p 寫的 P2P Docker 遠端部署工具。
每台機器都跑同一個 app，節點會透過 LAN、mDNS、relay、DHT 彼此發現，再由 Web UI 把 deployment bundle 推送到遠端節點執行 `docker compose up -d --build`。

目前只保留三塊核心能力：

- P2P 節點連線與發現
- P2P Docker 遠端部署
- Web UI 管理節點、bundle 與部署流程

## 安裝

需求：

- Go 1.25 或更新版本
- 若要跨 NAT，建議準備一台有 public IP 的 relay/bootstrap node
- 接收部署的節點要能執行 Docker
- Windows 接收方需要已安裝 WSL，且 WSL 內可執行 `docker`

下載 Go dependencies：

```bash
go mod tidy
```

建立本機資料夾：

```bash
mkdir -p bundles deployments
```

建立 `.env`：

```bash
cp .env.example .env
```

## 啟動

```bash
go run ./cmd/app
```

程式啟動時會自動讀取專案根目錄的 `.env`。如果同一個變數同時存在於 shell 環境與 `.env`，會以 shell 環境為準。

## Docker 遠端部署

1. 在接收部署的節點 `.env` 開啟：

   ```bash
   APP_DOCKER_DEPLOY_ENABLED=true
   ```

2. 建議再加上保護：

   ```bash
   APP_DOCKER_ALLOWED_PEERS=12D3KooW...
   APP_DOCKER_DEPLOY_TOKEN=replace-this-token
   ```

3. 如果要指定 gVisor：

   ```bash
   APP_DOCKER_RUNTIME=runsc
   ```

4. 如果接收方是 Windows，會自動透過 `wsl` 執行 Docker。也可以指定 WSL distro：

   ```bash
   APP_DOCKER_WSL_DISTRO=Ubuntu
   ```

5. 在操作端 Web UI 上傳 `.zip`、`.tar.gz`、`.tgz` 或 `.tar` bundle。bundle 內要包含 `docker-compose.yml` 與必要的 app 檔案。
6. 在 UI 選擇要部署的 bundle、project name、compose 檔路徑，再對目標節點按 `Deploy Bundle`。

遠端節點會把 bundle 解壓到 `deployments/`，然後執行：

```bash
docker compose -p <project> -f <compose-file> up -d --build
```

若接收方是 Windows，則會改成透過 `wsl` 執行同一條命令。

## 跨 NAT / Relay

在有 public IP 的 relay 機器上，`.env` 至少設定：

```bash
APP_RELAY_SERVICE=true
APP_P2P_PORT=4001
```

如果要手動公告 public address：

```bash
APP_ANNOUNCE_ADDRS=/ip4/203.0.113.10/tcp/4001
```

其他節點則設定：

```bash
APP_STATIC_RELAYS=/ip4/203.0.113.10/tcp/4001/p2p/12D3KooW...
APP_BOOTSTRAP_PEERS=/ip4/203.0.113.10/tcp/4001/p2p/12D3KooW...
```

## 環境變數

- `APP_WEB_HOST`：HTTP UI listen host，預設 `0.0.0.0`
- `APP_WEB_PORT`：HTTP UI port，預設 `3000`
- `APP_P2P_HOST`：libp2p listen host，預設 `0.0.0.0`
- `APP_P2P_PORT`：libp2p listen port，預設 `0`
- `APP_DISCOVERY_GROUP`：LAN multicast group，預設 `239.255.77.77`
- `APP_DISCOVERY_PORT`：LAN discovery UDP port，預設 `50197`
- `APP_MDNS_ENABLED=false`：關閉 libp2p mDNS discovery。預設開啟
- `APP_RELAY_SERVICE=true`：讓這個節點提供 circuit relay 和 AutoNAT service
- `APP_ANNOUNCE_ADDRS="<multiaddr>[,<multiaddr>]"`：手動公告 public address
- `APP_STATIC_RELAYS="<multiaddr>[,<multiaddr>]"`：private peer 可使用的 relay 節點
- `APP_BOOTSTRAP_PEERS="<multiaddr>[,<multiaddr>]"`：DHT 啟動時要連線的 bootstrap peer
- `APP_RENDEZVOUS="<namespace>"`：DHT rendezvous namespace
- `APP_DHT_ENABLED=false`：關閉 DHT discovery。預設開啟
- `APP_DHT_MODE=auto-server|auto|server|client`：DHT 模式
- `APP_ENABLE_HOLE_PUNCHING=false`：關閉 hole punching。預設開啟
- `APP_ENABLE_NAT_PORT_MAP=false`：關閉 UPnP / NAT-PMP port mapping。預設開啟
- `APP_FORCE_PRIVATE_REACHABILITY=false`：使用 static relay 時，不強制標記自己為 private reachability
- `APP_DOCKER_DEPLOY_ENABLED=true`：允許這台節點接收遠端 Docker 部署請求
- `APP_DOCKER_ALLOWED_PEERS="<peerId>[,<peerId>]"`：只有白名單中的 peer 可以部署到這台機器
- `APP_DOCKER_DEPLOY_TOKEN="<secret>"`：要求 deploy request 提供相同 token 才能執行
- `APP_DOCKER_RUNTIME="runsc"`：部署時指定 Docker runtime
- `APP_DOCKER_WSL_DISTRO="Ubuntu"`：接收方是 Windows 時，指定用哪個 WSL distro 執行 Docker
- `P2PTEST_KEY_PATH`：自訂 libp2p private key 路徑

## 專案結構

```text
.
├── cmd/
│   └── app/
│       ├── main.go
│       ├── web.go
│       ├── ui.go
│       ├── p2p.go
│       ├── docker.go
│       ├── discovery.go
│       ├── mdns.go
│       ├── dht.go
│       ├── nat.go
│       ├── state.go
│       ├── types.go
│       ├── config.go
│       ├── identity.go
│       ├── json.go
│       └── constants.go
├── bundles/
├── deployments/
├── go.mod
├── go.sum
└── README.md
```

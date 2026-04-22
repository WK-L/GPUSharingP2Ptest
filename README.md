# P2P 檔案傳輸

這是一個 Go/libp2p 寫的小型 P2P 檔案傳輸工具。每台電腦都跑同一個 app，在瀏覽器選擇 Sender 或 Receiver 後，就可以直接傳檔。

目前支援：

- LAN 內自動探索 receiver。
- NAT port mapping，也就是 UPnP / NAT-PMP。
- libp2p hole punching。
- libp2p circuit relay，讓跨 NAT 的節點可以透過 relay 傳輸。
- DHT rendezvous，讓 sender 不需要手動輸入 receiver address，也能找到已公告的 receiver。

## 安裝

需求：

- Go 1.25 或更新版本。
- 如果只測 LAN，兩台電腦要在同一個區網。
- 如果要跨網路 / 跨 NAT，建議準備一台有 public IP 的 VPS 當 relay/bootstrap node。

下載 Go dependencies：

```bash
go mod tidy
```

建立本機傳輸資料夾：

```bash
mkdir -p outbox received
```

資料夾用途：

- `outbox/`：sender 要傳出的檔案。
- `received/`：receiver 收到的檔案。

## 啟動

```bash
go run ./cmd/app
```

啟動後，終端機會印出：

- `Web UI`：本機瀏覽器開這個。
- `LAN Web UI`：同 LAN 其他電腦可以開這個。
- `Peer ID`：這個節點的 libp2p ID。
- `P2P address`：其他節點可以用來連線的 multiaddr。

## LAN 傳檔

1. 兩台電腦都執行：

   ```bash
   go run ./cmd/app
   ```

2. Receiver 那台在網頁切到 `Receiver`。
3. Sender 那台在網頁切到 `Sender`。
4. Sender 選檔案後按 `Add Files`。
5. 等 receiver 出現在清單後，按 `Send Files`。

LAN discovery 只會在同一個區網內生效。

## 單機雙開測試

只測基本 sender / receiver 流程時，一台電腦就可以開兩個 terminal 測試。兩個 instance 必須使用不同的 Web port、P2P port 和 key。app 預設會開啟 local discovery，讓同一台電腦上的 receiver 可以穩定出現在 sender 的 receiver list。

Sender：

```bash
APP_WEB_PORT=3000 APP_P2P_PORT=4001 go run ./cmd/app
```

Receiver：

```bash
APP_WEB_PORT=3001 APP_P2P_PORT=4002 go run ./cmd/app
```

開啟：

- `http://127.0.0.1:3000`
- `http://127.0.0.1:3001`

如果你手動指定 `P2PTEST_KEY_PATH`，請確認 sender 和 receiver 不要使用同一個 key 檔，否則兩邊會變成同一個 Peer ID，receiver 不會出現在 sender 的 receiver list。

## 跨 NAT 傳檔

跨不同網路時，需要至少一台 public relay/bootstrap node。最常見做法是用 VPS。

### 1. 啟動 public relay

在有 public IP、且 TCP port 有開的機器上執行：

```bash
APP_RELAY_SERVICE=true APP_P2P_PORT=4001 go run ./cmd/app
```

如果這台機器只印出 private IP，但你知道它的 public IP 或 DNS，可以手動公告 public address：

```bash
APP_RELAY_SERVICE=true \
APP_P2P_PORT=4001 \
APP_ANNOUNCE_ADDRS="/ip4/203.0.113.10/tcp/4001" \
go run ./cmd/app
```

把 relay 印出的 `P2P address` 複製起來，例如：

```text
/ip4/203.0.113.10/tcp/4001/p2p/12D3KooW...
```

### 2. 啟動 NAT 後面的節點

Sender 和 Receiver 都用同一個 relay address 啟動：

```bash
APP_STATIC_RELAYS="/ip4/203.0.113.10/tcp/4001/p2p/12D3KooW..." \
APP_BOOTSTRAP_PEERS="/ip4/203.0.113.10/tcp/4001/p2p/12D3KooW..." \
go run ./cmd/app
```

Receiver 切到 `Receiver` 後，會透過 DHT rendezvous 公告自己。Sender 切到 `Sender` 後，會透過 DHT 自動尋找 receiver，找到後會出現在 receiver 清單中。

如果 private peer 成功取得 relay reservation，UI 的 `Network` 區塊會出現 `/p2p-circuit` address，按 `Copy Circuit Address` 可以複製。你也可以把這個 address 貼到另一台 sender 的 `Send to Address` 手動傳送。

### 3. 建議測試順序

先用三個節點測：

- 一台 VPS：跑 relay/bootstrap。
- 一台家裡或公司網路的電腦：跑 receiver。
- 另一個不同網路的電腦：跑 sender。

如果 sender 看不到 receiver，先檢查：

- VPS 的 TCP `4001` 是否有開。
- Sender / Receiver 的 `APP_STATIC_RELAYS` 和 `APP_BOOTSTRAP_PEERS` 是否填同一個 relay address。
- UI 的 `Network` 區塊是否顯示 `DHT peers` 大於 0。
- Receiver 是否已經切到 `Receiver` 模式。

## 環境變數

- `APP_WEB_HOST`：HTTP UI listen host，預設 `0.0.0.0`。
- `APP_WEB_PORT`：HTTP UI port，預設 `3000`。
- `APP_P2P_HOST`：libp2p listen host，預設 `0.0.0.0`。
- `APP_P2P_PORT`：libp2p listen port，預設 `0`，代表自動分配。
- `APP_DISCOVERY_GROUP`：LAN multicast group，預設 `239.255.77.77`。
- `APP_DISCOVERY_PORT`：LAN discovery UDP port，預設 `50197`。
- `APP_LOCAL_DISCOVERY=false`：關閉單機雙開測試用的 local discovery。預設開啟。
- `APP_MDNS_ENABLED=false`：關閉 libp2p mDNS discovery。預設開啟。
- `APP_RELAY_SERVICE=true`：讓這個節點提供 circuit relay 和 AutoNAT service。通常只開在 public VPS。
- `APP_ANNOUNCE_ADDRS="<multiaddr>[,<multiaddr>]"`：手動公告 public address，適合雲端機器。
- `APP_STATIC_RELAYS="<multiaddr>[,<multiaddr>]"`：private peer 可使用的 relay 節點。
- `APP_BOOTSTRAP_PEERS="<multiaddr>[,<multiaddr>]"`：DHT 啟動時要連線的 bootstrap peer。
- `APP_RENDEZVOUS="<namespace>"`：DHT rendezvous namespace，預設 `/gpusharingp2ptest/files/receiver/v1`。
- `APP_DHT_ENABLED=false`：關閉 DHT discovery。預設開啟。
- `APP_DHT_MODE=auto-server|auto|server|client`：DHT 模式。relay 預設 `server`，其他節點預設 `auto-server`。
- `APP_ENABLE_HOLE_PUNCHING=false`：關閉 hole punching。預設開啟。
- `APP_ENABLE_NAT_PORT_MAP=false`：關閉 UPnP / NAT-PMP port mapping。預設開啟。
- `APP_FORCE_PRIVATE_REACHABILITY=false`：使用 static relay 時，不強制標記自己為 private reachability。

## 專案結構

```text
.
├── cmd/
│   └── app/
│       ├── main.go        # app 啟動、libp2p host 組裝
│       ├── web.go         # HTTP API 和 web server
│       ├── ui.go          # 內嵌瀏覽器 UI
│       ├── p2p.go         # libp2p stream handler 和傳檔 dial helper
│       ├── discovery.go   # LAN multicast/broadcast receiver discovery
│       ├── local_discovery.go # 單機雙開測試用 local discovery
│       ├── mdns.go        # libp2p mDNS discovery
│       ├── dht.go         # DHT rendezvous 自動節點發現
│       ├── nat.go         # relay、AutoRelay、hole punching、bootstrap 設定
│       ├── files.go       # outbox / received 檔案處理
│       ├── state.go       # 共享狀態和 API state response
│       ├── types.go       # request / response / state 型別
│       ├── config.go      # 環境變數、address、網址 helper
│       ├── identity.go    # libp2p private key 載入與建立
│       ├── json.go        # JSON response helper
│       └── constants.go   # protocol、資料夾、interval 常數
├── outbox/                # sender 待傳檔案
├── received/              # receiver 收到的檔案
├── go.mod
├── go.sum
└── README.md
```

## 常用指令

啟動 app：

```bash
go run ./cmd/app
```

跑測試：

```bash
go test ./cmd/app
```

build binary：

```bash
go build -o app ./cmd/app
```

# FRP 通信序列化改造与多平台构建指南

## 1. 目标与结果

本文档记录本次改造的完整过程与可复用命令，覆盖以下内容：

- 将 `frp server/client` 的消息通信从原 JSON 控制器切换到自定义 `jydiy` 控制器。
- 新序列化格式采用 `gob`，反序列化采用 `gob` 对应解码。
- 在 macOS (Mac Studio, arm64) 上完成交叉编译，产出：
  - Linux amd64: `frps`, `frpc`
  - Windows amd64: `frpc.exe`

---

## 2. 改造核心思路（本质与机制）

### 2.1 本质

本次改造只替换了消息编解码控制层（`msgCtl`），不改业务消息结构体和上层调用方式。

也就是说，`server` 和 `client` 仍然通过统一入口调用：

- `msg.ReadMsg(...)`
- `msg.ReadMsgInto(...)`
- `msg.WriteMsg(...)`

但这些入口的底层实现从 `golib/msg/json` 切换到了 `pkg/msg/jydiy`。

### 2.2 机制

消息在网络中的二进制格式为：

- `Type`：1 字节，消息类型
- `Length`：8 字节，大端 `uint64`
- `Payload`：N 字节，`gob` 编码后的消息体

即：`[1-byte type][8-byte length][payload]`

读取流程：

1. 先读 1 字节类型。
2. 校验类型是否已注册。
3. 读 8 字节长度。
4. 校验长度合法（`>=0` 且不超过 `maxMsgLength`）。
5. 读完整 payload。
6. 用 `gob.NewDecoder(...).Decode(...)` 反序列化。

写入流程：

1. 根据消息类型反查 `typeByte`。
2. 用 `gob.NewEncoder(...).Encode(...)` 序列化 payload。
3. 组包为 `type + length + payload`。
4. 一次写出。

---

## 3. 改动文件清单

### 3.1 修改文件

- `pkg/msg/ctl.go`

变更点：

- import 从 `github.com/fatedier/golib/msg/json` 改为 `github.com/fatedier/frp/pkg/msg/jydiy`
- `Message` 类型别名、`msgCtl` 实例类型、`NewMsgCtl()` 来源同步切换到 `jydiy`
- 对外 API (`ReadMsg`, `ReadMsgInto`, `WriteMsg`) 保持不变

### 3.2 新增文件

- `pkg/msg/jydiy/ctl.go`
- `pkg/msg/jydiy/pack.go`
- `pkg/msg/jydiy/process.go`

职责分工：

- `ctl.go`：控制器结构、注册类型映射
- `pack.go`：`Pack/UnPack/UnPackInto` 与 `gob` 编解码
- `process.go`：流式 `ReadMsg/WriteMsg` 与长度校验、错误定义

---

## 4. 与原实现相比的关键变化

- 上层调用不变：业务代码不需要改。
- 线协议 payload 从 JSON 文本改为 Gob 二进制。
- 协议头结构仍保持 `type + length + content`，迁移风险可控。
- `ReadMsg`/`WriteMsg` 的调用点（server/client/proxy/control 等）全部自动生效。

注意：

- Gob 与 JSON 不兼容，通信双方必须同时使用同一协议版本。

---

## 5. Go 版本选择与原因

### 5.1 选择版本

- 最终使用：`go1.25.8`

### 5.2 原因

- 项目 `go.mod` 声明：`go 1.25.0`
- 本机旧版本 `go1.21.3` 会触发 toolchain 自动下载失败或版本不满足问题
- 升级到 `1.25.8` 后可直接使用 `GOTOOLCHAIN=local` 本地构建

### 5.3 版本检查

```bash
go version
go env GOVERSION GOTOOLCHAIN
```

---

## 6. 依赖库安装方式（Go 模块）

本项目使用 Go Modules，通常不需要手动逐个安装库。

### 6.1 推荐方式

```bash
cd /Users/ljh/Documents/workspace_go/frp
GOTOOLCHAIN=local go mod download
```

### 6.2 构建时自动下载

即使不提前 `go mod download`，首次 `go build` 也会自动下载 `go.mod` 中声明依赖并写入模块缓存。

---

## 7. 编译命令（含参数解释）

以下命令均在仓库根目录执行：

```bash
cd /Users/ljh/Documents/workspace_go/frp
```

### 7.1 Linux amd64（frps + frpc）

```bash
mkdir -p dist/linux_amd64
GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags noweb -o dist/linux_amd64/frps ./cmd/frps
GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags noweb -o dist/linux_amd64/frpc ./cmd/frpc
```

### 7.2 Windows amd64（frpc.exe）

```bash
mkdir -p dist/windows_amd64
GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags noweb -o dist/windows_amd64/frpc.exe ./cmd/frpc
```

如需 `frps.exe`，可用：

```bash
GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags noweb -o dist/windows_amd64/frps.exe ./cmd/frps
```

### 7.3 参数含义

- `GOTOOLCHAIN=local`：强制使用本机已安装 Go，不走自动下载 toolchain。
- `CGO_ENABLED=0`：关闭 CGO，便于生成静态、可移植交叉编译产物。
- `GOOS=linux/windows`：目标操作系统。
- `GOARCH=amd64`：目标架构 x86_64。
- `-tags noweb`：禁用内嵌 web 静态资源，避免 `web/*/dist` 缺失导致构建失败。
- `-o`：指定输出文件路径。
- `./cmd/frps`、`./cmd/frpc`：指定要编译的主程序入口。

### 7.4 一条命令同时编译三个产物（本次实测成功）

适用场景：

- 当前分支使用 `go:embed dist`（需要先构建 `web/frps/dist`、`web/frpc/dist`）。
- 本机已安装 `Node.js >= 20.19`（本次为 `v22.22.1`）和 `Go 1.25.8`。

```bash
cd /Users/ljh/Documents/workspace_go/frp && node -v && npm -v && cd web/frps && npm install && npm run build && cd ../frpc && npm install && npm run build && cd ../.. && mkdir -p dist/linux_amd64 dist/windows_amd64 && GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux_amd64/frps ./cmd/frps && GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux_amd64/frpc ./cmd/frpc && GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows_amd64/frpc.exe ./cmd/frpc && ls -lh dist/linux_amd64 dist/windows_amd64 && file dist/linux_amd64/frps dist/linux_amd64/frpc dist/windows_amd64/frpc.exe
```

该命令会一次完成：

- 构建 `frps` 面板静态资源
- 构建 `frpc` 面板静态资源
- 交叉编译 Linux amd64: `frps`、`frpc`
- 交叉编译 Windows amd64: `frpc.exe`
- 输出产物大小与文件类型校验结果

---

## 8. 产物位置与验证

### 8.1 当前产物

- `dist/linux_amd64/frps`
- `dist/linux_amd64/frpc`
- `dist/windows_amd64/frpc.exe`

### 8.2 验证文件类型

```bash
file dist/linux_amd64/frps dist/linux_amd64/frpc
file dist/windows_amd64/frpc.exe
```

---

## 9. 常见问题

### 9.1 报错：`pattern dist: no matching files found`

原因：

- `web/frps/embed.go` 和 `web/frpc/embed.go` 默认会 embed `dist` 目录。

解决：

- 构建时加 `-tags noweb`，使用对应的 `embed_stub.go`。

### 9.2 报错：自动下载 Go toolchain 失败

解决：

- 安装与 `go.mod` 匹配的 Go 版本（本次为 `1.25.8`）。
- 构建时显式加：`GOTOOLCHAIN=local`。

---

## 10. 你特别要求的提醒命令

```bash
GOTOOLCHAIN=local go build ./cmd/frps ./cmd/frpc
```

说明：

- 该命令用于本地直接构建默认目标平台。
- 若你要交叉编译到 Linux/Windows amd64，请使用第 7 节中的带 `GOOS/GOARCH` 命令。

---

## 11. 可选的后续优化建议

- 增加一键脚本：`build_release.sh`，统一输出 Linux/Windows 产物。
- 在 CI 中固定 Go `1.25.x`，避免环境漂移。
- 若未来需要协议灰度，可在握手阶段增加协议版本字段并做双栈兼容。

---

## 12. 生产验证与前后对比（证明改造成功）

本节给出两套可落地的验证方案，建议都做：

- 方案 A：黑盒联调 + 网络抓包对比（最快证明线上通信已切换）
- 方案 B：白盒回归 + 编解码一致性测试（最稳妥证明机制正确）

建议验收标准：

- 功能可用：`frpc` 能稳定登录、心跳正常、代理可用。
- 协议已切换：抓包中 payload 不再是 JSON 文本可读内容，而是 Gob 二进制。
- 稳定性可接受：长连接持续运行无异常断连、无反序列化错误。

### 12.1 方案 A：黑盒联调 + 抓包对比（推荐先做）

目标：

- 从运行行为和网络字节特征两方面，证明协议从 JSON 变为 Gob。

步骤 1：准备两套二进制（旧版与新版）

- 旧版：改造前 commit 编译出的 `frps/frpc`
- 新版：本次改造后的 `frps/frpc`

步骤 2：在同一测试环境跑两轮对比

- 轮次 1（基线）：旧版 `frps + frpc`
- 轮次 2（改造后）：新版 `frps + frpc`
- 保持配置文件一致、网络环境一致、压测脚本一致

步骤 3：抓包并导出样本

在 `frps` 所在机（或中间链路）执行：

```bash
sudo tcpdump -i any -s 0 -w /tmp/frp_new.pcap tcp port 7000
```

如果你也抓基线包：

```bash
sudo tcpdump -i any -s 0 -w /tmp/frp_old.pcap tcp port 7000
```

步骤 4：判定协议是否切换

- 旧版（JSON）通常在 payload 中可看到明显文本特征，如 `{`, `"proxy_name"`, `"run_id"`。
- 新版（Gob）payload 为二进制，不再具备稳定的 JSON 文本可读特征。

可用命令做快速侧证（不是唯一证据）：

```bash
strings /tmp/frp_old.pcap | grep -E 'proxy_name|run_id|new_proxy|login' | head
strings /tmp/frp_new.pcap | grep -E 'proxy_name|run_id|new_proxy|login' | head
```

预期：

- 旧版匹配结果明显更多；新版显著减少或几乎没有。

步骤 5：功能与稳定性验收

- 登录：`frpc` 启动后能成功连接 `frps`。
- 心跳：持续运行期间无周期性断连。
- 代理：选 2 到 3 个典型代理（如 tcp/http/stcp）验证转发正常。
- 运行时日志：无 `message type error`、`message format error`、`decode` 等异常。

建议压测窗口：

- 冒烟：10 到 15 分钟
- 生产前：至少 2 小时稳定运行

### 12.2 方案 B：白盒回归 + 编解码一致性测试（建议纳入 CI）

目标：

- 证明 `Pack -> ReadMsgInto` 与 `WriteMsg -> ReadMsg` 在所有核心消息类型上可逆、一致。

验证要点：

- 每个消息类型都能注册并正确映射 `typeByte`。
- 序列化后再反序列化，关键字段一致。
- 非法输入能触发预期错误（长度越界、未知类型等）。

建议新增测试点（可在 `pkg/msg/jydiy` 下实现）：

1. `TestPackUnpack_Login`
2. `TestPackUnpack_NewProxy`
3. `TestPackUnpack_UDPPacket`
4. `TestReadMsg_UnknownType`
5. `TestReadMsg_LengthExceeded`
6. `TestReadMsg_TruncatedPayload`

示意测试流程：

1. 构造消息对象（如 `Login`）。
2. `WriteMsg(bytes.Buffer, msg)` 写入。
3. `ReadMsgInto(bytes.Reader, &dst)` 读取。
4. 比较关键字段。
5. 构造坏数据并断言错误类型。

执行命令：

```bash
cd /Users/ljh/Documents/workspace_go/frp
GOTOOLCHAIN=local go test ./pkg/msg/jydiy -v
GOTOOLCHAIN=local go test ./pkg/msg/... -v
```

CI 建议：

- 每次提交强制跑 `pkg/msg/...`。
- 每次发布前增加一次 `frps+frpc` 集成联调作业（可用 docker compose）。

### 12.3 兼容性与回滚演练（生产强烈建议）

由于 Gob 与 JSON 不兼容，生产必须重点验证“版本匹配”与“回滚可用”。

建议做 4 组组合测试：

1. 旧 `frps` + 旧 `frpc`（应成功，作为基线）
2. 新 `frps` + 新 `frpc`（应成功，本次目标）
3. 旧 `frps` + 新 `frpc`（预期失败，验证不兼容行为可观测）
4. 新 `frps` + 旧 `frpc`（预期失败，验证不兼容行为可观测）

需要记录：

- 失败时日志特征（便于现场快速识别版本不匹配）
- 回滚时间（从新版本切回旧版本的总耗时）
- 回滚后业务恢复时间（代理恢复、连接恢复）

### 12.4 推荐的最终验收结论模板

可在发布记录中写成：

1. 已完成黑盒抓包对比：新版本 payload 非 JSON 文本特征，符合 Gob 协议预期。
2. 已完成白盒编解码回归：核心消息类型 `Pack/UnPack` 一致性通过。
3. 已完成新新组合联调与 2 小时稳定性运行：无异常断连与反序列化错误。
4. 已完成不兼容组合演练与回滚演练：行为可观测，回滚路径可执行。

满足以上 4 条，可判定“序列化改造已成功且具备生产发布条件”。

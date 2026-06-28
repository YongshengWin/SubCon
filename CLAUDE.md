# CLAUDE.md

## 项目概述

SubCon (surge-sub-converter) 是一个独立的 Go 订阅转换服务，从 3x-ui 面板拉取原始代理订阅链接，转换为 Surge / Clash / Stash / Shadowrocket / Quantumult X 等客户端可用的配置格式。不依赖 3x-ui 数据库，也不修改面板代码。

- **Go 版本**: 1.22
- **模块名**: `surge-sub-converter`
- **依赖**: 纯标准库，无第三方依赖
- **入口**: `main.go` → `main()`，单一二进制，内嵌前端页面

## 构建与运行

```bash
# 开发运行
go run . 

# 构建（注入版本号）
go build -trimpath -ldflags="-s -w -X main.version=<ver> -X main.userAgent=surge-sub-converter/<ver>" -o surge-sub-converter .

# 运行测试
go test ./...

# 指定端口
SSC_PORT=8090 go run .
```

版本通过 `main.version` / `main.userAgent` 这两个包级变量注入，默认值 `"dev"`。

## 项目结构

| 文件 | 职责 |
|---|---|
| `main.go` | HTTP 服务、协议解析（VMess/VLESS/Trojan/SS/Hysteria2/Snell）、订阅拉取、缓存、短链管理、节点注册 API、各目标客户端渲染 |
| `templates.go` | 前端 HTML 页面（Go `html/template` 内嵌），提供订阅转换 UI |
| `clash_template.go` | Clash 配置 YAML 模板，包含 DNS、规则集（Google/YouTube/Bilibili 等）和策略组骨架 |
| `main_test.go` | 单元测试，覆盖短链 API、各协议解析、Shadowrocket/Clash 渲染、Hysteria2 |
| `auto_snell.sh` | Snell v5 一键部署脚本，支持 systemd/OpenRC/Alpine，自动注册到 SubCon |
| `snell-docker/Dockerfile` | Snell-server Docker 镜像（备用方案，Alpine 256MB RAM 下不推荐） |
| `install.sh` | 源码安装（需 Go），编译并注册 systemd 服务到 `/opt/surge-sub-converter` |
| `install-release.sh` | 发布版安装（无需 Go），下载预编译二进制；`sub 1` 更新命令的入口 |
| `package-release.sh` | 交叉编译 linux/amd64 + linux/arm64 并打包 tar.gz |
| `publish-release.sh` | 上传 release assets 到 GitHub Releases |
| `uninstall.sh` | 卸载脚本 |

## 路由与 API

| 路径 | 方法 | 用途 |
|---|---|---|
| `/` | GET | 前端页面 |
| `/healthz` | GET | 健康检查，返回 `ok` |
| `/convert` | GET | 直接转换链接（**已禁用公开访问**，返回 403） |
| `/api/convert` | POST | JSON API，接收 `{url, target, policy, udp, skip_cert_verify, direct, list}` 返回转换结果 |
| `/api/shorten` | POST | 创建/更新短链，接收 `{target, url, existingShort}`，返回 `{shortUrl}` |
| `/s/<token>` | GET | 短链出口，根据 User-Agent 自动切换代理客户端模式（list-only） |
| `/api/node` | POST/PUT/DELETE | Snell 节点注册/更新/删除，需 HMAC 签名（`X-Timestamp` + `X-Signature`） |
| `/api/snell-short` | POST | 创建 Snell 专属短链（仅加载本地 snell_nodes.txt，不走外部订阅源） |

## 支持的协议

节点解析入口 `parseProxy()` (`main.go:554`)，按前缀分发：

- `vmess://` — Base64 JSON payload
- `vless://` — URL 格式
- `trojan://` — URL 格式
- `ss://` — 多种编码格式（SIP002 等）
- `hysteria2://` / `hy2://` — URL 格式，支持 obfs/pinSHA256
- `snell://` — URL 格式 `snell://psk@host:port?version=N&obfs=http&obfs-host=x#name`

## Snell 协议关键知识

### 版本兼容性

Snell v5 服务端向下兼容 v4 客户端。客户端配 `version=5` 会触发 QUIC Proxy Mode（需 UDP），常规 TCP 部署必须用 `version=4`。

**正确方案**：服务端 v5.0.1 二进制 + 客户端 `version=4`。

### Alpine/musl 兼容

snell-server v5 是 glibc 编译 + UPX 压缩的。Alpine 需要：

1. `apk add gcompat upx` — gcompat 提供 glibc 兼容，upx 解压二进制
2. `upx -d /usr/local/bin/snell-server` — 解压 UPX 壳（否则内核只看到 UPX stub）
3. `mkdir -p /lib64 && ln -sf /lib/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2` — glibc 二进制硬编码了 `/lib64` 路径

`auto_snell.sh` 已自动处理以上三步。

### 服务管理

- **systemd** (Debian/Ubuntu): `systemctl restart snell`
- **OpenRC** (Alpine): `rc-service snell restart`，使用 `supervise-daemon` 监管前台进程

## 目标客户端渲染

`renderByTarget()` (`main.go:918`) 根据 target 参数分发：

- `surge` → `[Proxy]` / `[Proxy Group]` / `[Rule]` 三段式 INI
- `clash` / `stash` → YAML（模板注入 proxies + proxy-groups + rules）
- `shadowrocket` → Base64 编码的节点链接列表
- `quantumultx` → `[server_local]` + `[policy]` + `[filter_local]` 格式（Snell 不支持 QX）

## Snell 节点注册系统

### 自动注册流程 (`auto_snell.sh`)

1. 检测系统架构（amd64/aarch64/armv7l）
2. 抓取 Snell 最新版本号（从 Surge 官方页，fallback v5.0.1）
3. 智能端口分配（10000-60000，检测占用）
4. 生成随机 PSK（16 字符）
5. 获取公网 IP + 交互式输入域名前缀（拼接 `xxx.115emby.top`）
6. 下载 → 解压 → 安装 snell-server
7. Alpine 兼容处理（UPX 解压 + gcompat + /lib64 软链）
8. 写入 `/etc/snell/snell-server.conf` + `.registration_info`
9. 创建 systemd / OpenRC 服务并启动
10. HMAC 签名注册到 SubCon（JSON body 含 host/port/psk/version/name/node_id）

### 复用检测

重跑脚本时自动检测已有配置，复用端口/PSK，仅更新二进制并重新注册。node_id 确保跨 host 改名不产生重复记录。

### 持久节点 ID

`node_id` 是 32 字符 hex，首次运行时生成并持久化到 `.registration_info`。注册/更新时 API 优先按 `node_id` 匹配，即使 host/域名改变也能正确更新而非新增。

### API 安全

节点注册需 HMAC-SHA256 签名：`HMAC(timestamp|body, secret)`，timestamp 允许 ±300s 偏差，防重放攻击。

## 关键设计决策

- **缓存**: 内存缓存（`converterCache`），60s TTL，以 SHA1(subURL|target|policy|testURL|udp|skip_cert_verify|direct) 为 key
- **短链存储**: 文件系统 `/opt/surge-sub-converter/data/subscriptions.txt`，格式 `token|title|target|url`
- **节点存储**: 文件系统 `/opt/surge-sub-converter/data/snell_nodes.txt`，每行一个 snell URI
- **Snell 专属短链**: `local://snell-nodes` sentinel URL，`convertSubscription` 遇到此 URL 跳过外部拉取，仅加载本地节点
- **安全措施**: 订阅 URL 拉取前校验 DNS 解析结果，阻止内网/环回地址；拉取日志仅打印 host 不打印完整 URL
- **并发拉取**: 多订阅源 URL 通过 goroutine + WaitGroup 并发抓取
- **节点去重**: 基于 "名称|类型|主机|端口|配置项" 指纹去重，重名节点自动追加数字后缀

## Surge 链式代理配置

在 Surge 配置中通过 `underlying-proxy` 实现中转：

```
[Proxy]
🔗 AWS-JP = snell, azjp.115emby.top, 31883, psk=xxx, version=4
🔗 Bread = snell, breadjp.115emby.top, 38545, psk=yyy, version=4, underlying-proxy=🔗 AWS-JP

[Proxy Group]
🇯🇵 落地 = select, 🔗 Bread, 🔗 Legend, update-interval=0
```

用户 Mac → AWS Snell 中转 → 落地 Snell → 目标。AWS 出站 IPv4 可能被安全组限制，需在 AWS 控制台放行。

## 环境变量

| 变量 | 默认值 |
|---|---|
| `SSC_LISTEN` | `0.0.0.0` |
| `SSC_PORT` | `8090` |
| `SSC_TEST_URL` | `http://www.gstatic.com/generate_204` |
| `SSC_FETCH_TIMEOUT` | `15`（秒） |
| `SSC_CACHE_TTL` | `60`（秒） |
| `SSC_USER_AGENT` | `surge-sub-converter/<version>` |
| `SSC_CERT_FILE` / `SSC_KEY_FILE` | 空（不启用 TLS） |
| `SSC_LINKS_FILE` | `/opt/surge-sub-converter/data/subscriptions.txt` |
| `SSC_NODES_FILE` | `/opt/surge-sub-converter/data/snell_nodes.txt` |
| `SSC_NODE_SECRET` | 空（未设置时 `/api/node` 返回 403） |

## 常见问题排查

### Snell 节点不通

1. **端口监听**: `ss -tlnp | grep snell` 确认服务在监听
2. **云防火墙**: 云控制台安全组需放行对应 TCP 端口
3. **PSK 一致**: `grep psk /etc/snell/snell-server.conf` 与订阅输出对比
4. **Alpine crash**: `rc-service snell status` 看是否 stopped/crashed，`journalctl -u snell` 没有 journal，用 `tail /var/log/messages | grep snell`
5. **AWS VPC**: AWS EC2 出站 IPv4 TCP 默认可能被安全组限制，需在 Outbound rules 放行

### sub 1 更新

DMIT 服务器上执行 `sub 1` 自动从 GitHub Releases 拉最新版并重启。手动更新：
```bash
curl -fsSL -o /tmp/sc.tar.gz "https://github.com/YongshengWin/SubCon/releases/download/v1.x.x/surge-sub-converter-linux-amd64.tar.gz"
tar -xzf /tmp/sc.tar.gz -C /tmp && mv /tmp/surge-sub-converter /usr/local/bin/
systemctl restart surge-sub-converter
```

## CI/CD

GitHub Actions (`.github/workflows/release.yml`)：推送 `v*` tag 时触发，执行 `package-release.sh` 交叉编译 linux/amd64 和 linux/arm64，通过 `softprops/action-gh-release@v2` 发布到 GitHub Releases。

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
| `main.go` | HTTP 服务、协议解析（VMess/VLESS/Trojan/SS/Hysteria2）、订阅拉取、缓存、短链管理、各目标客户端渲染 |
| `templates.go` | 前端 HTML 页面（Go `html/template` 内嵌），提供订阅转换 UI |
| `clash_template.go` | Clash 配置 YAML 模板，包含 DNS、规则集（Google/YouTube/Bilibili 等）和策略组骨架 |
| `main_test.go` | 单元测试，覆盖短链 API、各协议解析、Shadowrocket/Clash 渲染、Hysteria2 |
| `install.sh` | 源码安装（需 Go），编译并注册 systemd 服务到 `/opt/surge-sub-converter` |
| `install-release.sh` | 发布版安装（无需 Go），下载预编译二进制 |
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

## 支持的协议

节点解析入口 `parseProxy()` (`main.go:554`)，按前缀分发：

- `vmess://` — Base64 JSON payload
- `vless://` — URL 格式
- `trojan://` — URL 格式
- `ss://` — 多种编码格式（SIP002 等）
- `hysteria2://` / `hy2://` — URL 格式，支持 obfs/pinSHA256

## 目标客户端渲染

`renderByTarget()` (`main.go:918`) 根据 target 参数分发：

- `surge` → `[Proxy]` / `[Proxy Group]` / `[Rule]` 三段式 INI
- `clash` / `stash` → YAML（模板注入 proxies + proxy-groups + rules）
- `shadowrocket` → Base64 编码的节点链接列表
- `quantumultx` → `[server_local]` + `[policy]` + `[filter_local]` 格式

## 关键设计决策

- **缓存**: 内存缓存（`converterCache`），60s TTL，以 SHA1(subURL|target|policy|testURL|udp|skip_cert_verify|direct) 为 key
- **短链存储**: 文件系统 `/opt/surge-sub-converter/data/subscriptions.txt`，格式 `token|title|target|url`
- **安全措施**: 订阅 URL 拉取前校验 DNS 解析结果，阻止内网/环回地址；拉取日志仅打印 host 不打印完整 URL
- **并发拉取**: 多订阅源 URL 通过 goroutine + WaitGroup 并发抓取
- **节点去重**: 基于 "名称|类型|主机|端口|配置项" 指纹去重，重名节点自动追加数字后缀

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

## CI/CD

GitHub Actions (`.github/workflows/release.yml`)：推送 `v*` tag 时触发，执行 `package-release.sh` 交叉编译 linux/amd64 和 linux/arm64，通过 `softprops/action-gh-release@v2` 发布到 GitHub Releases。

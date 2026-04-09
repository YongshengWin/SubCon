# 订阅转换小工具

这是一个独立的 Go 服务，适合部署在你的服务器上，给 3x-ui 的原始订阅补一个订阅转换出口。

它包含两部分：

- 后端转换接口
- 一个可直接使用的前端页面
- 一个 `sub` 命令行管理面板

不依赖 3x-ui 数据库，也不需要改动 3x-ui 面板代码。

## 当前支持

- **节点协议**：`VMess` `VLESS` `Trojan` `Shadowsocks (SS/SSD)`
- **目标客户端**：`Surge` `Clash` `Stash` `Shadowrocket` `Quantumult X`

内置多种专业分流规则集（ChatGPT, Google, YouTube, Bilibili 等），实现国内外流量一键分流。

## 页面与接口

启动后可访问：

- `/` 前端页面
- `/healthz` 健康检查
- `/api/convert` 返回 JSON，供前端预览调用
- `/s/<token>` 返回目标客户端对应的纯文本配置短链

## 发布版安装

推荐使用发布版安装脚本，目标机不需要 Go：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/YongshengWin/SubCon/main/install-release.sh)
```

如果仓库信息不是默认值，也可以显式传入：

```bash
REPO_OWNER=YongshengWin REPO_NAME=SubCon REPO_BRANCH=main bash install-release.sh
```

这个脚本会自动获取最新 release，对应下载二进制包，不在服务器现场编译。

相关文件：

- `install-release.sh`
- `uninstall.sh`
- `package-release.sh`

## 源码安装

服务器需要先安装 Go 1.22 或更高版本。

```bash
chmod +x install.sh
sudo APP_PORT=8090 ./install.sh
```

安装脚本会：

1. 把源码复制到 `/opt/surge-sub-converter`
2. 编译生成 `/usr/local/bin/surge-sub-converter`
3. 注册并启动 `systemd` 服务

如果你要自己发布，请先本地打包：

```bash
cd tools/surge_sub_converter
chmod +x package-release.sh
./package-release.sh
```

然后把生成的这两个文件上传到你的发布地址：

- `surge-sub-converter-linux-amd64.tar.gz`
- `surge-sub-converter-linux-arm64.tar.gz`

默认 release 地址格式是：

```text
https://github.com/YongshengWin/SubCon/releases/download/<tag>/
```

默认会自动通过 GitHub API 获取最新 tag。

如果你想手动指定版本，也可以覆盖：

```bash
VERSION=v0.1.0 bash install-release.sh
```

如果你的下载地址或 API 地址不同，安装时可以覆盖：

```bash
BASE_URL=https://你的下载地址 API_URL=https://你的API地址 bash install-release.sh
```

常用命令：

```bash
sub
sub 1
sub 3
sub uninstall
sudo systemctl status surge-sub-converter
sudo systemctl restart surge-sub-converter
sudo journalctl -u surge-sub-converter -f
```

卸载：

```bash
sub uninstall
```

## sub 管理面板

安装后可直接执行：

```bash
sub
```

面板支持 ANSI 彩色输出，菜单按功能分组：

```text
── 基础管理 ─────────────────────────
 1.  更新服务
 2.  查看服务状态
 7.  重启服务
 8.  卸载服务

── HTTPS / 域名 ─────────────────────
 6.  HTTPS / 域名设置

── 订阅管理 ─────────────────────────
 3.  查看订阅链接
 4.  添加转换订阅链接
 5.  删除转换订阅链接

11. 绑定公网 IP
 0.  退出
```

### HTTPS / 域名设置（菜单 6）

选择 `6` 后进入 HTTPS 子菜单：

1. **复用已有证书（推荐）** — 适合 3x-ui 共存场景
2. **仅设置域名** — 配合 Cloudflare CDN 使用
3. **Nginx 反代** — 自动生成 nginx 配置片段
4. **Caddy 反代** — 高级选项

### 与 3x-ui 共存的 HTTPS 配置

如果你已经安装了 3x-ui，80/443 端口通常已被 xray 占用。推荐使用子菜单的 **选项 1（复用已有证书）**：

- 自动探测 `/root/cert/` 和 `~/.acme.sh/` 下的证书文件
- 转换服务默认监听 `8443` 端口（Cloudflare 兼容）
- 复用 acme.sh 或 3x-ui 已有的证书文件，无需重新申请

配置完成后，访问地址变为：

```text
https://sub.example.com:8443/s/<token>
```

Cloudflare 橙云代理支持的 HTTPS 端口：`443, 2053, 2083, 2087, 2096, 8443`。

如需证书续签后自动重载转换服务：

```bash
acme.sh --install-cert -d 你的域名 --reloadcmd "systemctl restart surge-sub-converter"
```

如果你的服务器无法稳定通过外部服务探测公网 IP，可以使用第 11 项手动绑定公网 IP。
之后 `sub 3` 和管理面板顶部显示的访问地址，会优先使用这个绑定 IP。


## 使用方式

假设你 3x-ui 的原始订阅地址是：

```text
http://your-server:2096/sub/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

建议直接打开前端页面：

```text
http://your-converter-server:8090/
```

在页面里粘贴原始订阅 URL，然后：

- 预览生成的配置
- 生成长期可用的短链
- 复制或直接打开短链结果

出于安全考虑，公开入口不再推荐也不再展示带原始订阅 URL 的明文转换链接。

## 可选参数

- `target=surge|clash|stash|quantumultx` 指定目标客户端
- `policy=Proxy` 自定义策略组名称
- `udp=true` 是否启用 `udp-relay`
- `skip_cert_verify=true` 是否输出 `skip-cert-verify=true`
- `direct=true` 是否把 `DIRECT` 加入策略组

## 资源优化

当前版本已做这些轻量优化：

- 转换结果增加了内存缓存，减少重复回源和重复解析
- `sub` 命令不再依赖 `python3`，也不再默认外呼公网 IP 查询服务
- `sub 2` 改成摘要状态看板，直接显示运行状态、版本、地址、TLS 模式和最近日志
- `test_url=http://www.gstatic.com/generate_204` 自定义测速 URL

## 环境变量

- `SSC_LISTEN` 默认 `0.0.0.0`
- `SSC_PORT` 默认 `8090`
- `SSC_TEST_URL` 默认 `http://www.gstatic.com/generate_204`
- `SSC_FETCH_TIMEOUT` 默认 `15`，单位秒
- `SSC_USER_AGENT` 默认 `surge-sub-converter/0.2`

## 说明

这个工具的职责只有两件事：

1. 拉取 3x-ui 原始订阅
2. 输出目标客户端配置

如果你后面还想继续扩展 Clash、Sing-box、Stash，可以继续在这个 Go 服务上往下加。

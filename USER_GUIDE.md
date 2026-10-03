# jlu-drcom-win 使用指南

## 初次配置

发布包只包含托盘程序 `drcom-tray.exe`、配置模板和文档。复制 `config.example.toml` 为 `config.toml`，填写账号密码后双击程序即可自动登录。配置路径默认相对于 exe，而非当前工作目录。先退出其他占用 UDP 61440 的校园网客户端。

```toml
username = "你的校园网账号"
password = "你的校园网密码"
```

请勿分享真实配置。密码中含反斜杠或双引号时，应遵守 TOML 转义规则，也可使用单引号字面量字符串（内容不能包含单引号）。未知键、重复键和错误类型会被明确拒绝。

## 网卡与高级配置

默认自动读取网卡 IPv4/MAC，并排除名称中可识别的常见虚拟网卡。存在多个同等候选网卡时不会任意挑选，请设置名称或唯一名称片段：

```toml
adapter_hint = "以太网"
```

可以同时手动配置 `ip` 和 `mac`。只手动指定其中一项时，必须与自动探测到的同一张网卡一致，否则报错。手动固定 IP 的配置在 DHCP 地址变化后需要更新，因此优先推荐 `adapter_hint`。

`bind_ip` 应保持默认 `auto`，或等于认证 IP，不接受通配地址。`primary_dns` 只是认证报文字段，**不会修改 Windows DNS**。

数值范围：`receive_timeout_ms` 为 100..30000，`retry_count` 为 0..10，`heartbeat_interval_seconds` 为 1..300；正常使用保持默认 2000、3、20。完整字段见配置模板。

## 托盘菜单

- `Login`：重新读取配置并开始登录。
- `Logout`：停止自动重连，尽力下线；下线清理总时限为 1.5 秒。
- `Start with Windows`：开启/关闭当前用户登录自启。
- `Open configuration`：打开当前配置文件；缺失时先从模板创建。
- `Open log folder`：打开当前用户的日志目录。
- `Exit`：停止认证并退出。

`Authenticated` 在登录和首轮心跳校验成功后显示，只代表认证协议成功；`Reconnecting` 表示等待网络恢复，重试间隔逐步增长到最多 60 秒。`Failed - check config/logs; then Login` 表示需要查看日志并修正配置或账号，再点击 `Login`，不会无限提交错误密码。

修改已在线程序的配置后，先 `Logout` 再 `Login`；未登录时直接 `Login`。双击重复启动不会产生第二个实例。主动下线后，睡眠唤醒不会擅自重新登录；原本希望保持认证的实例会在恢复后重新检测网络。

## 开机自启

通过托盘菜单启用当前用户登录计划任务，无人为启动延迟，使用普通用户权限，不保存 Windows 密码。旧版 Run 注册表启动项会在任务注册成功后迁移；失败时保留原项并记录错误。

计划任务记录 exe 和配置的绝对路径。移动程序后，先取消再重新勾选自启。不要从 `go run` 的临时路径设置自启。管理员策略禁止创建任务时菜单操作会提示错误。

启动任务不等待 Explorer 就绪；认证在后台独立进行。实际启动耗时取决于 Windows 调度、网卡和校园服务器，需要下次用户登录后通过日志确认，不保证登录桌面前完成认证。

## 日志与故障排查

日志文件为 `%LOCALAPPDATA%\jlu-drcom-win\logs\tray.log`，运行中约每 4 MiB 轮转，保留当前文件与一份 `.old`。日志包含 IP、网卡名及错误等诊断信息，分享前仍应检查个人信息。

兼容旧配置项 `debug_hex_dump = true`，但它现在只增加协议阶段、请求/响应长度，不再输出原始报文。正常情况下不需要开启。

```powershell
Get-Content "$env:LOCALAPPDATA\jlu-drcom-win\logs\tray.log" -Tail 30
Get-NetUDPEndpoint -LocalPort 61440 -ErrorAction SilentlyContinue
Get-ScheduledTask | Where-Object TaskName -Like 'jlu-drcom-tray-*'
```

持续超时应检查校园网连接、网卡选择、防火墙和服务器地址。认证成功但网页打不开，应单独排查系统 DNS、浏览器代理和 Clash 规则；不要仅凭托盘状态判断互联网连通。参见仓库内 `docs/NETWORK_TROUBLESHOOTING.md`。

## 更新和回退

先从托盘退出旧程序，再替换 exe 和公共文档，保留自己的 `config.toml`。新版不再提供 CLI；已有旧 CLI 不会被构建脚本自动删除，请勿同时运行。旧配置若显式使用 `bind_ip = "0.0.0.0"`，需移除此项或改成 `auto`。

建议更新前保留旧 exe。远程协作依赖 Clash 时，不要直接关闭 Clash；必须切换 TUN 的测试应有独立本地恢复进程，并提前验证恢复路径。

# jlu-drcom-win

吉林大学校园网 Dr.COM 认证客户端，面向 Windows 的轻量托盘程序。仅交付 `drcom-tray.exe`，早期 CLI 已移除。

## 使用

1. 解压发布包，把 `config.example.toml` 复制为 `config.toml`，填写校园网账号和密码。
2. 退出其他 Dr.COM 客户端，双击 `drcom-tray.exe`，程序自动登录。
3. 点击托盘图标管理登录、下线、开机自启，或打开配置与日志目录。

```toml
username = "你的校园网账号"
password = "你的校园网密码"
# 多网卡环境建议明确指定校园网网卡：
# adapter_hint = "以太网"
```

配置默认从 exe 所在目录读取。日志位于 `%LOCALAPPDATA%\jlu-drcom-win\logs`，不要求安装目录可写。

## 行为与边界

- 使用当前用户登录计划任务自启，无人为延迟；网卡未就绪时后台重试。
- 认证 UDP 绑定校园网 IPv4，并在 Windows 上指定出接口；不自动更改 DNS、全局路由或 Clash 设置。
- 按请求类型、心跳阶段及序号匹配应答，过滤迟到或无关报文，退出可中断正在等待的收包。
- 重连前重新读取配置和网卡信息；连续故障按 5、10、20、40、60 秒退避，认证恢复后重置。
- 配置错误或服务器拒绝登录时停止自动重试，修正后点击 `Login`。
- `Authenticated` 表示登录及心跳验证成功，不保证 DNS、代理或所有网页可用。
- 日志只记录协议阶段、长度、结果，不输出报文、密码派生值或会话令牌；运行中轮转日志。
- 单实例运行，支持 Explorer 重启后的图标恢复、休眠恢复重连，以及有时限的退出/注销清理。

详见 [使用指南](USER_GUIDE.md)、[网络排查](docs/NETWORK_TROUBLESHOOTING.md)、[审查记录与修复状态](docs/CODE_REVIEW_2026-10-03.md)。

## 构建

Windows，Go 1.22 或更高版本：

```powershell
.\scripts\test-build.ps1
.\scripts\build.ps1
```

脚本运行测试和 vet，只在成功后更新 `dist\jlu-drcom-win.zip`。不会清理或覆盖已解压运行目录中的用户配置。发布包不含真实账号或日志。

[开发说明](docs/DEVELOPMENT.md) · [发布流程](docs/RELEASE.md) · [协议字段](docs/PROTOCOL_FIELDS.md)

## 限制与致谢

账号密码仍以明文保存在本地配置中，请限制该文件访问权限。默认图标为系统图标；没有 Windows Service，不能在用户登录前显示托盘。网卡筛选依赖名称和显式配置，无法覆盖所有第三方虚拟网卡命名。真实关机、休眠、多网卡切换与无网开机仍需在各自机器上验证。

协议知识参考原 C 项目 [AndrewLawrence80/jlu-drcom-client](https://github.com/AndrewLawrence80/jlu-drcom-client)，保留已验证的协议构包与校验算法。按 CC BY-NC-SA 4.0 发布，见 [LICENSE](LICENSE) 和 [NOTICE.md](NOTICE.md)。

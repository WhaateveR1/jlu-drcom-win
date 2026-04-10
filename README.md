# jlu-drcom-win

吉林大学校园网 Dr.COM 认证客户端 Windows 版。

这是一个面向 Windows 日常使用的重构版客户端：保留原 C 项目里已经验证过的协议字段和校验算法，重新实现配置、UDP 收发、心跳、下线、重连和托盘操作。

## 下载和使用

从 GitHub Releases 下载 `jlu-drcom-win.zip`，解压后运行：

```powershell
Copy-Item config.example.toml config.toml
notepad .\config.toml
.\drcom-tray.exe -config .\config.toml
```

`config.toml` 里只需要填写：

```toml
username = "你的校园网账号"
password = "你的校园网密码"
```

程序会自动读取当前物理网卡的 IPv4、MAC 和电脑名。完整说明见 [USER_GUIDE.md](USER_GUIDE.md)。

## 当前功能

- Windows 托盘程序：登录、下线、退出、状态展示、开机自启。
- 命令行程序：适合首次配置和查看日志。
- 自动读取当前物理网卡 IPv4 和 MAC。
- 登录、双心跳保活、超时重试。
- Ctrl+C 或托盘退出时发送下线包。
- 心跳失败后关闭旧 socket、重新绑定并重新登录。
- 发布包不包含本地 `config.toml`。

## 命令行

只测试登录：

```powershell
.\drcom-win.exe -config .\config.toml -login-only
```

登录并持续保活：

```powershell
.\drcom-win.exe -config .\config.toml
```

## 构建

```powershell
go test ./...
.\scripts\build.ps1
```

发布包：

```text
dist\jlu-drcom-win.zip
```

GitHub 发布流程见 [docs/RELEASE.md](docs/RELEASE.md)。

## 项目结构

```text
cmd/
  drcom-win/      命令行入口
  drcom-tray/     托盘入口

internal/
  config/         配置解析、默认值、网卡自动探测
  protocol/       协议包构造、解析、校验算法
  transport/      UDP socket、timeout、来源校验
  runner/         登录、心跳、下线、重连状态机
  trayapp/        Windows 托盘程序
  logging/        日志和 hex dump
```

开发说明见 [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)，协议字段表见 [docs/PROTOCOL_FIELDS.md](docs/PROTOCOL_FIELDS.md)。

## 致谢

本项目的协议字段整理和早期验证参考了原 C 项目 [AndrewLawrence80/jlu-drcom-client](https://github.com/AndrewLawrence80/jlu-drcom-client)。本项目不是逐行移植，而是在保留协议知识的基础上用 Go 重写 Windows 客户端运行模型。

## 许可

本项目按 CC BY-NC-SA 4.0 发布，见 [LICENSE](LICENSE)。

# jlu-drcom-win 用户使用指南

## 这是什么

`jlu-drcom-win` 是吉林大学校园网 Dr.COM 认证客户端的 Windows 版。

发布包里有两个程序：

- `drcom-tray.exe`：托盘版，适合日常使用。
- `drcom-win.exe`：命令行版，适合第一次调试或查看错误日志。

日常使用优先运行 `drcom-tray.exe`。

## 使用前准备

解压发布包后，目录里应当有这些文件：

```text
drcom-tray.exe
drcom-win.exe
config.example.toml
README.md
USER_GUIDE.md
LICENSE
NOTICE.md
CHANGELOG.md
```

复制一份配置文件：

```powershell
Copy-Item config.example.toml config.toml
```

然后用记事本打开 `config.toml`，只填写账号和密码。

## 填写配置

必须填写：

```toml
username = "你的校园网账号"
password = "你的校园网密码"
```

程序会自动读取当前物理网卡的 IPv4、MAC 和电脑名。其他配置都有默认值。

如果自动选错网卡，再手动指定。常见写法：

```toml
adapter_hint = "以太网"
```

也可以直接写死 IP 和 MAC：

```toml
ip = "192.168.1.100"
mac = "00:11:22:33:44:55"
```

高级字段一般不用改：

```toml
server_ip = "10.100.61.3"
auth_version = "6800"
keepalive_version = "dc02"
first_heartbeat_version = "0f27"
extra_heartbeat_version = "db02"
```

查看本机 IP：

```powershell
Get-NetIPConfiguration | Where-Object { $_.IPv4Address -and $_.NetAdapter.Status -eq 'Up' } |
  Select InterfaceAlias,InterfaceDescription,@{n='IPv4';e={$_.IPv4Address.IPAddress}}
```

查看网卡 MAC：

```powershell
Get-NetAdapter | Where-Object { $_.Status -eq 'Up' } |
  Select Name,InterfaceDescription,MacAddress,Status
```

自动选择通常会排除 Hyper-V、WSL、VMware、VirtualBox 这类虚拟网卡。多网卡环境下如果选错，用 `adapter_hint` 指定物理网卡名称的一部分。

## 先退出旧客户端

本程序需要绑定本地 UDP `61440` 端口。旧 Dr.COM 客户端运行时通常也会占用这个端口。两个客户端不能同时运行。

检查端口占用：

```powershell
Get-NetUDPEndpoint -LocalPort 61440 -ErrorAction SilentlyContinue |
  Select LocalAddress,LocalPort,OwningProcess
```

如果有输出，查看占用程序：

```powershell
Get-Process -Id <OwningProcess>
```

测试本程序前，先退出旧 Dr.COM 客户端。

## 推荐使用托盘版

在解压目录运行：

```powershell
.\drcom-tray.exe -config .\config.toml
```

启动后，右下角托盘区会出现一个程序图标。如果没看到，点任务栏右侧的上箭头。

点击托盘图标打开菜单：

- `Status`：查看当前状态。
- `Login`：登录校园网并开始保活。
- `Logout`：下线并停止保活。
- `Start with Windows`：开启或关闭开机自启。
- `Exit`：退出托盘程序。在线时会先下线再退出。

常见状态含义：

```text
Stopped          未运行
LoginChallenge   正在请求登录 salt
LoggingIn        正在登录
Online           已在线
Reconnecting     正在重连
LoggingOut       正在下线
Failed           运行失败
```

## 开机自启

托盘菜单里的 `Start with Windows` 会给当前用户添加开机自启项。

它使用的是当前 `drcom-tray.exe` 路径和当前 `config.toml` 路径。移动程序目录后，需要重新点一次 `Start with Windows`。

检查自启项：

```powershell
Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' |
  Select jlu-drcom-tray
```

## 命令行版调试

第一次配置时，建议先用命令行版确认能登录。

只登录一次，成功后立即退出：

```powershell
.\drcom-win.exe -config .\config.toml -login-only
```

正常输出会包含：

```text
config loaded
udp socket bound
login challenge started
login challenge succeeded
login succeeded
login flow completed
```

持续运行并保活：

```powershell
.\drcom-win.exe -config .\config.toml
```

正常运行时会持续出现：

```text
heartbeat succeeded
```

退出命令行版按 `Ctrl+C`。程序会发送下线包，正常输出包含：

```text
logout challenge started
logout challenge succeeded
logout succeeded
runner stopped
```

## 常见问题

### 端口被占用

现象：

```text
open udp transport: bind: Only one usage of each socket address ...
```

原因是已有程序占用了 UDP `61440`。退出旧 Dr.COM 客户端或另一个正在运行的 `drcom-win.exe` / `drcom-tray.exe`。

### 登录 challenge 超时

常见原因：

- 旧客户端没有完全退出，端口或认证状态冲突。
- 当前网络没有连接到校园网认证环境。
- `server_ip` 填错。
- Windows 防火墙拦截 UDP。
- 自动选错网卡，或手动填写的 IP/MAC 不对。

先检查日志里的 `adapter` 字段。如果选到了虚拟网卡，在 `config.toml` 里加 `adapter_hint`，或者直接手动填写 `ip` 和 `mac`。

### 登录成功后很快断线

用命令行版观察是否持续出现：

```text
heartbeat succeeded
```

如果出现 `reconnect attempt started`，说明程序已经在自动重连。连续重连失败时，重点检查网络是否切换、电脑是否睡眠唤醒、IP/MAC 是否已经变化。

### 托盘图标启动了但菜单打不开

使用最新的 `drcom-tray.exe`。旧版本曾有托盘点击消息兼容问题。

### 开机自启没有生效

确认是运行打包后的 `drcom-tray.exe` 后开启自启。不要用 `go run` 开启自启，因为 `go run` 的程序路径是临时目录。

## 调试日志

需要看协议包时，把配置改成：

```toml
debug_hex_dump = true
```

然后用命令行版运行：

```powershell
.\drcom-win.exe -config .\config.toml
```

登录包里的密码派生字段会脱敏，但日志仍会包含用户名、IP、MAC。

## 当前限制

- 密码保存在本地 `config.toml`。
- 托盘版使用系统默认图标。
- 没有 Windows Service 模式。

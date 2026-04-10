# Protocol Field Table

字段来自原项目 `jlu-drcom-client/client.c`。长度单位为字节，区间采用左闭右开。

## Challenge

Login challenge 和 logout challenge 都是 20 字节。

| Offset | Length | Login | Logout | Meaning |
| --- | ---: | --- | --- | --- |
| 0 | 1 | `01` | `01` | challenge marker |
| 1 | 1 | `02` | `03` | login/logout subtype |
| 2 | 2 | random | random | request random bytes |
| 4 | 2 | auth version | auth version | default `68 00` |
| 6 | 14 | zero | zero | padding |

响应 salt 位于 `[4:8)`。login 响应类型为 `02 02`，logout 响应类型为 `02 03`。

## Login

`SIZE_LOGIN = 334 + ((len(password) - 1) / 4) * 4`。

| Offset | Length | Meaning |
| --- | ---: | --- |
| 0 | 2 | `03 01` |
| 2 | 1 | `00` |
| 3 | 1 | `len(username) + 20` |
| 4 | 16 | `md5(03 01 || login_salt || password)` |
| 20 | 36 | username |
| 56 | 1 | control check status, default `00` |
| 57 | 1 | adapter num, default `00` |
| 58 | 6 | MAC xor MD5A |
| 64 | 16 | `md5(01 || password || login_salt || 00 00 00 00)` |
| 80 | 1 | IP indicator, `01` |
| 81 | 4 | client IPv4 |
| 97 | 8 | first 8 bytes of `md5(packet[0:97] || 14 00 07 0b)` |
| 105 | 1 | IP dog, default `01` |
| 110 | 32 | host name |
| 142 | 4 | primary DNS |
| 146 | 4 | DHCP server |
| 181 | 1 | `01` |
| 182 | 8 | `44 72 43 4f 4d 00 cf 07` |
| 190 | 2 | auth version |
| 192 | 54 | OS info |
| 246 | 40 | fixed unknown indicator copied from original client |
| 310 | 2 | auth version |
| 313 | 1 | password length |
| 314 | 0..16 | `ROR((password xor MD5A)[0:min(len(password),16)])` |
| `314+n` | 2 | `02 0c`, where `n = min(len(password),16)` |
| `316+n` | 4 | checksum |
| `322+n` | 6 | MAC |
| `328+n+padding` | 2 | random trailer |

Checksum input is `packet[0:316+n] || 01 26 07 11 00 00 || mac` and is zero-padded by the Go implementation when the length is not divisible by 4.

## KeepAlive Auth

固定 38 字节。

| Offset | Length | Meaning |
| --- | ---: | --- |
| 0 | 1 | `ff` |
| 1 | 16 | login MD5A |
| 17 | 3 | zero |
| 20 | 16 | server Dr.COM indicator from login response `[23:39)` |
| 36 | 2 | Unix timestamp low 16 bits, little-endian |

## Heartbeat

固定 40 字节。

| Offset | Length | Meaning |
| --- | ---: | --- |
| 0 | 1 | `07` |
| 1 | 1 | heartbeat count low byte |
| 2 | 3 | `28 00 0b` |
| 5 | 1 | phase, `01` for first/extra/step1, `03` for step2 |
| 6 | 2 | heartbeat version |
| 8 | 4 | random token |
| 16 | 4 | server heartbeat token |
| 24 | 4 | step2 CRC |
| 28 | 4 | client IPv4 in step2 |

版本字段：

| Packet | Version |
| --- | --- |
| first heartbeat | `first_heartbeat_version`, default `0f 27` |
| extra heartbeat | `extra_heartbeat_version`, default `db 02` |
| step1/step2 | `keepalive_version`, default `dc 02` |

原 C 项目在 step1 random token 处写 `random_token[8..11]`，Go 版改为正确写 4 字节 token。

## Logout

固定 80 字节。

| Offset | Length | Meaning |
| --- | ---: | --- |
| 0 | 2 | `06 01` |
| 2 | 1 | `00` |
| 3 | 1 | `len(username) + 20` |
| 4 | 16 | `md5(06 01 || logout_salt || password)` |
| 20 | 36 | username |
| 56 | 1 | control check status, default `00` |
| 57 | 1 | adapter num, default `00` |
| 58 | 6 | MAC xor MD5A |
| 64 | 16 | server Dr.COM indicator |

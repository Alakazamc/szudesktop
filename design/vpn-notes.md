# 深信服 EasyConnect 协议分析笔记

本笔记记录协议流程的分析结果，`internal/vpn` 的代码据此**独立编写**。
协议行为来自对客户端与服务端交互的观察以及公开的技术资料，
**未复制、未包含任何厂商或第三方的源代码**，也不引入 GPL / AGPL 授权的代码。

目标：在 `internal/vpn` 中实现，供桌面端（--app UI）和 CLI 共用。
适用服务器：深大 `ssl.szu.edu.cn` / `svpn.szu.edu.cn`。

> 本项目与深信服科技股份有限公司不存在任何关联、合作或授权关系。
> EasyConnect 及相关名称的一切权利归深信服所有。仅供个人学习与研究使用。

## 协议全流程（五步）

1. **Web 登录拿 TWFID**
   - `GET https://<server>/por/login_auth.csp?apiversion=1` → XML 里抓
     `TwfID`、`RSA_ENCRYPT_KEY`（hex 模数）、`RSA_ENCRYPT_EXP`（缺省 65537）、`CSRF_RAND_CODE`。
   - 密码先拼上 CSRF：`password = password + "_" + csrfCode`，再 RSA PKCS1v15 加密，hex 编码。
   - `POST /por/login_psw.csp?anti_replay=1&encrypt=1&type=cs`，Cookie `TWFID=<id>`，
     form：`svpn_name` / `svpn_password`(hex) / `svpn_req_randcode`(csrf) / `svpn_rand_code`="" / `mitm`=""。
   - 响应 `<Result>1</Result>` = 成功；`<NextService>auth/sms</NextService>` 或 `<NextAuth>2</NextAuth>` = 要短信验证码
     （`POST /por/login_sms.csp` 发送 → `POST /por/login_sms1.csp` 提交 `svpn_inputsms`）；
     `auth/token` / `totp` = 要 TOTP（`POST /por/login_token.csp` 提交 `svpn_inputtoken`）。
   - 全程 TLS 不校验证书（学校自签）。

2. **ECAgent token（最骚的一步）**
   - 用 uTLS 明文发两个 HTTP 请求（`/por/conf.csp` + `/por/rclist.csp`，带 TWFID Cookie）到 443。
   - **TLS ServerHello 的 SessionId 字段就是 token 前半段**：`hex(SessionId)[:31] + "\x00"`（31 字节 + NUL）。
   - 需要 uTLS 才能读到握手层原始数据（标准库 crypto/tls 拿不到 SessionId 原文）。

3. **拼 48 字节 token**：`token = agentToken(31+1) + TWFID(16)`。

4. **特制 TLS 通道**（VPN 数据通道和 HTTPS 共用 443，靠 ClientHello 区分）：
   - uTLS HelloCustom：ClientRandom 随机、TLS 1.1、CipherSuite 固定 `TLS_RSA_WITH_RC4_128_SHA`、
     **SessionId 前 4 字节 = 'L','3','I','P'，其余 0**（服务端看到这个就知道是 VPN 通道）。
   - `QueryIp`：写 `[0x00 00 00 00] + token + [00*8 ff ff ff ff]`，回包 `[0]=0`，`reply[4:8]` = 分配的内网 IP。
     **这条连接不能关**，关了后面收发握手会失败。
   - 收流：写 `[0x06 00 00 00] + token + [00*8] + ipReversed`，回 `[0x01]` 开头即成功，之后持续读裸 IPv4 包。
   - 发流：写 `[0x05 00 00 00] + token + [00*8] + ipReversed`，回 `[0x02]` 开头即成功，之后把内核栈交来的
     IPv4 包原样写进这条 TLS。断线各自重连（fork 是重试 5 次）。

5. **用户态 TCP/IP 栈 + SOCKS5**：
   - 隧道里跑的是裸 IPv4 帧，要自己终结 TCP → gVisor netstack（`gvisor.dev/gvisor`）。
   - 自定义 LinkEndpoint：`WriteTo` 把收到的包塞给 dispatcher；`WritePackets` 回调把栈发出的包交给 TX 流。
   - 栈配置：IPv4 + TCP/UDP、本机 IP/32、默认路由、SACK + cubic。
   - SOCKS5（CONNECT，无认证）收到目标地址 → `net.ResolveIPAddr`（DNS 走本地，校内域名解析要 VPN 通了才对——
     **注意**：DNS 是个坑，校外解析内网域名会失败，v2 可把 DNS 查询也走隧道或用服务器下发 DNS）。
   - fork 用 tailscale.com/net/socks5，我们自写 ~100 行省掉这个巨型依赖。

## 依赖与体积

- `github.com/refraction-networking/utls`（必须，读 SessionId + 特制 ClientHello）
- `gvisor.dev/gvisor`（netstack，重，二进制会涨十几 MB，可接受）
- 拒绝：tailscale（只为一个 socks5 不值得）。代理源 `goproxy.cn` 已验证可达。

## 与 fork 的差异（我们的改造）

- 日志走回调（`SetLogger`），UI 直接显示，不用彩色 stdout。
- 生命周期用 context：`Start(ctx)` 可取消，RX/TX 重试循环响应 ctx（fork 用 panic）。
- SOCKS5 只监听 127.0.0.1，避免开放代理。
- 状态机：Disconnected → LoggingIn → (NeedSMS/NeedTOTP) → Connecting → Connected(IP) → 断开重连。
- 二进制计划：先 SOCKS5 模式（简单、无需管理员），虚拟网卡 TUN 全局模式以后再说（要 wintun.dll + 路由操作）。

## 风险

- 深信服协议是逆向出来的，学校服务端固件升级随时可能失效（原作者已停维护，fork 仍在）。
- 校外才能测：在校内连 ssl.szu.edu.cn 行为未定义，真机测试得在校外/热点环境做。
- 合规：仅用本人账号连自己学校的 VPN，代码注释里写明来源与用途。

## 待验证

- [ ] ssl.szu.edu.cn 的 login_auth.csp 响应字段是否和 fork 预期一致（字段名/CSRF 有无）
- [ ] ECAgent SessionId 截取 31 字节是否适配深大设备
- [ ] QueryIp 是否直接发内网 IP（深大可能下发的是 WebVPN 资源池 IP）

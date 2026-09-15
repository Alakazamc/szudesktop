# -*- coding: utf-8 -*-
"""在真·校园网里体检深澜链路（只读，不发登录请求、不碰账号密码）。

验证顺序对应代码里 Login() 的真实流程：
  1) 门户域名能不能解析 / 门户页能不能打开
  2) 页面内嵌的 acid 到底是多少（代码用正则从 ac_id=1 入口抓）
  3) 换 ac_id=12 入口，acid 会不会变
  4) get_challenge 拿不拿得到 token（登录第一步，不需要密码）
  5) 外网 204 探测 + 宿舍门户可达性（判断现在算哪个区）

注意：全程禁用系统代理。开着代理会把 net.szu.edu.cn 的解析和请求劫走，
      这也是 detect.go 里 Proxy: nil 的原因。
"""
import re
import socket
import ssl
import urllib.request

HOST = "https://net.szu.edu.cn"
DORM = "http://172.30.255.42"
UA = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"}


def client():
    # ProxyHandler({}) = 明确不走系统代理；证书不校验（校园网门户常用自签）
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    return urllib.request.build_opener(
        urllib.request.ProxyHandler({}),
        urllib.request.HTTPSHandler(context=ctx),
    )


def get(op, url, timeout=8):
    try:
        r = op.open(urllib.request.Request(url, headers=UA), timeout=timeout)
        return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return 0, ("ERR: %s" % e).encode()


def main():
    op = client()

    print("=== 1. 域名解析 ===")
    try:
        ips = socket.getaddrinfo("net.szu.edu.cn", None)
        print("   net.szu.edu.cn ->", sorted({a[4][0] for a in ips}))
    except Exception as e:
        print("   解析失败:", e)

    print("\n=== 2/3. 门户页里的 acid（代码从 ac_id=1 入口抓） ===")
    for entry in ("1", "12"):
        st, body = get(op, HOST + "/srun_portal_pc?ac_id=%s&theme=proyx" % entry)
        txt = body.decode("utf-8", "ignore")
        hits = re.findall(r"acid\s*[:=]\s*['\"]?(\d+)", txt)
        hits2 = re.findall(r"ac_id\s*[:=]\s*['\"]?(\d+)", txt)
        print("   入口 ac_id=%-2s HTTP %s  页面长度 %d" % (entry, st, len(txt)))
        print("      acid= 命中:", (hits[:6] or "无"))
        print("      ac_id= 命中:", (hits2[:6] or "无"))

    print("\n=== 4. get_challenge（登录第一步，不需要密码） ===")
    st, body = get(op, HOST + "/cgi-bin/get_challenge?callback=_&username=probe&ip=")
    print("   HTTP", st, "->", body.decode("utf-8", "ignore")[:300])

    print("\n=== 5. 区域判断用到探测 ===")
    st, body = get(op, "http://connect.rom.miui.com/generate_204")
    print("   外网 204 探测: HTTP %s  （204=已认证/能上网，非 204=被网关劫持=未认证）" % st)
    st2, _ = get(op, DORM + "/")
    print("   宿舍门户 172.30.255.42: %s" % ("可达" if st2 else "不可达 (%s)" % st2))
    print("   教学门户 net.szu.edu.cn: %s" % ("可达" if st else "见上"))


if __name__ == "__main__":
    main()

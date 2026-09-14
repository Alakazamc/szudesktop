import urllib.request

urls = [
    "https://cdn.jsdelivr.net/gh/abmasud1214/pufferdle@master/src/Fonts/svbold.ttf",
    "https://cdn.jsdelivr.net/gh/abmasud1214/pufferdle@main/src/Fonts/svbold.ttf",
    "https://cdn.jsdelivr.net/gh/abmasud1214/pufferdle@master/src/Fonts/svthin.ttf",
    "https://cdn.jsdelivr.net/npm/@vp-tw/cjk-web-fonts-fusion-pixel-font@0.0.1/dist/12px/proportional/zh_hans/Fusion-Pixel-12px-Proportional-Simplified-Chinese.css",
    "https://cdn.jsdelivr.net/npm/@fontsource/fusion-pixel-12px-proportional-sc@5.3.0/400.css",
    "https://cdn.jsdelivr.net/npm/@fontsource/press-start-2p@5.1.0/400.css",
    "https://cdn.jsdelivr.net/npm/@fontsource/fusion-pixel-12px-proportional-sc@5.3.0/files/fusion-pixel-12px-proportional-sc-chinese-simplified-1-normal.woff2",
]
for u in urls:
    try:
        req = urllib.request.Request(u, method="HEAD")
        with urllib.request.urlopen(req, timeout=12) as r:
            print(r.status, r.headers.get("content-type", ""), u.split("/")[-1], sep="  |  ")
    except Exception as e:
        print("FAIL", type(e).__name__, getattr(e, "code", ""), u)

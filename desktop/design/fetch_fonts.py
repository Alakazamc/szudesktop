import os, urllib.request, re

OUT = r"D:\szudesktop\desktop\assets\fonts"
os.makedirs(OUT, exist_ok=True)

SV = "https://cdn.jsdelivr.net/gh/abmasud1214/pufferdle@master/src/Fonts/"

files = {
    "svbold.ttf": SV + "svbold.ttf",
    "svthin.ttf": SV + "svthin.ttf",
}

for name, url in files.items():
    p = os.path.join(OUT, name)
    try:
        with urllib.request.urlopen(url, timeout=30) as r:
            data = r.read()
        with open(p, "wb") as f:
            f.write(data)
        print("OK  %-12s %8d bytes  %s" % (name, len(data), p))
    except Exception as e:
        print("FAIL", name, type(e).__name__, e)

# 看看融合像素字体的 CSS 里到底引用了哪些 woff2
css_url = ("https://cdn.jsdelivr.net/npm/@vp-tw/cjk-web-fonts-fusion-pixel-font@0.0.1/"
           "dist/12px/proportional/zh_hans/Fusion-Pixel-12px-Proportional-Simplified-Chinese.css")
try:
    with urllib.request.urlopen(css_url, timeout=30) as r:
        css = r.read().decode("utf-8")
    print("\n--- CSS ---")
    print(css[:1500])
    urls = re.findall(r"url\(([^)]+)\)", css)
    print("\nwoff2 count:", len(urls))
    for u in urls[:3]:
        print("  ", u)
except Exception as e:
    print("CSS FAIL", type(e).__name__, e)

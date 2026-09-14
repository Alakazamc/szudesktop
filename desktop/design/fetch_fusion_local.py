"""把融合像素字体（Fusion Pixel, OFL-1.1）整套拉到本地，替换 CDN，实现完全离线。
   - 解析官方 CSS 里的 @font-face（带 unicode-range 的分片 woff2）
   - 把 url(...) 全部改成本地相对路径，落盘 assets/fonts/fusion-pixel.css
   - 多线程下载，网络慢/失败就放弃并保留 CDN 方案
"""
import os, re, sys, urllib.request
from concurrent.futures import ThreadPoolExecutor

BASE = ("https://cdn.jsdelivr.net/npm/@vp-tw/cjk-web-fonts-fusion-pixel-font@0.0.1/"
        "dist/12px/proportional/zh_hans/")
CSS_NAME = "Fusion-Pixel-12px-Proportional-Simplified-Chinese.css"
OUT = r"D:\szuNet\desktop\assets\fonts"
os.makedirs(OUT, exist_ok=True)

req = urllib.request.Request(BASE + CSS_NAME, headers={"User-Agent": "Mozilla/5.0"})
css = urllib.request.urlopen(req, timeout=60).read().decode("utf-8")
files = sorted(set(re.findall(r"url\(([^)]+\.woff2)\)", css)))
print("css ok, woff2 files:", len(files))

def get(name):
    p = os.path.join(OUT, name)
    if os.path.exists(p) and os.path.getsize(p) > 0:
        return "cached"
    for attempt in range(3):
        try:
            r = urllib.request.Request(BASE + name, headers={"User-Agent": "Mozilla/5.0"})
            with urllib.request.urlopen(r, timeout=90) as resp:
                data = resp.read()
            with open(p, "wb") as f:
                f.write(data)
            return "ok %dB" % len(data)
        except Exception as e:
            err = "%s %s" % (type(e).__name__, getattr(e, "code", ""))
    return "FAIL " + err

results = {}
with ThreadPoolExecutor(max_workers=10) as ex:
    for name, res in zip(files, ex.map(get, files)):
        results[name] = res

bad = [k for k, v in results.items() if v.startswith("FAIL")]
print("downloaded:", len(files) - len(bad), "failed:", len(bad))
for k in bad[:8]:
    print("  FAIL", k, results[k])

if bad:
    print("SKIP-LOCAL: 有分片没下齐，继续用 CDN")
    sys.exit(2)

local_css = css.replace("url(", "url(")  # 相对路径本身就指向同目录，只要 css 和 woff2 放一起即可
with open(os.path.join(OUT, "fusion-pixel.css"), "w", encoding="utf-8") as f:
    f.write(local_css)
total = sum(os.path.getsize(os.path.join(OUT, n)) for n in files)
print("LOCAL-OK files=%d total=%.1f KB" % (len(files), total / 1024))

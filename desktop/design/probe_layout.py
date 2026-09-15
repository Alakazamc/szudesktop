"""量出页面每个区块的真实高度，判断"要不要滚动才能看全"。

做法：把页面复制一份、注入一段探针脚本，用无头 Edge 打开，
探针量完把结果塞进 document.title，再用 --dump-dom 读回来。

为什么用 file:// 而不是起服务：
  这个探针页就放在 desktop/design/ 下，旁边没有 assets/，
  所以必须是 desktop/ 那一层……不对，其实页面的相对路径是按页面所在目录算的，
  放 design/ 下会找不到 assets/art。

  所以这里换个办法：不复制页面，直接用 file:// 打开**原始页面**，
  通过 --dump-dom 拿不到注入的脚本。于是改成临时把 index.html 复制到
  desktop/ 下再注入，路径就对了。

用法: python probe_layout.py [宽] [高]
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

DESKTOP = r"D:\szuNet\desktop"
SRC = os.path.join(DESKTOP, "index.html")

PROBE = """
<script>
window.addEventListener('load', function () {
  setTimeout(function () {
    var out = { viewport: window.innerWidth + 'x' + window.innerHeight,
                docScrollH: document.documentElement.scrollHeight,
                bodyH: document.body.scrollHeight,
                blocks: [], sel: [] };
    document.querySelectorAll('body > *').forEach(function (el) {
      var r = el.getBoundingClientRect();
      out.blocks.push(el.tagName + '.' + (el.className || '-')
        + ' y=' + Math.round(r.top) + ' h=' + Math.round(r.height));
    });
    var want = ['.wrap', 'header', '.hero', '.tabs', '.dock', 'footer',
                '.panel', '.talk', '.ground', '#farm'];
    want.forEach(function (s) {
      var el = document.querySelector(s);
      if (!el) { out.sel.push(s + ' (没有)'); return; }
      var r = el.getBoundingClientRect();
      out.sel.push(s + ' y=' + Math.round(r.top) + ' h=' + Math.round(r.height)
        + ' w=' + Math.round(r.width));
    });
    // 溢出排查：内容实际需要的高度 > 自身可见高度 = 有东西被 overflow:hidden 裁掉
    out.ovf = [];
    document.querySelectorAll('body, body *').forEach(function (el) {
      var sh = el.scrollHeight, r = el.getBoundingClientRect();
      if (sh - Math.round(r.height) > 8 && sh < 20000) {
        out.ovf.push(el.tagName + '.' + (el.className || '-') + ' id=' + (el.id || '-')
          + ' 可见=' + Math.round(r.height) + ' 内容=' + sh);
      }
    });
    // main.wrap 直接子元素明细：找 scrollHeight 溢出的具体来源
    document.querySelectorAll('main.wrap > *').forEach(function (el) {
      var r = el.getBoundingClientRect(), cs = getComputedStyle(el);
      out.ovf.push('[子] ' + el.tagName + '.' + (el.className || '-') + ' id=' + (el.id || '-')
        + ' y=' + Math.round(r.top) + ' h=' + Math.round(r.height) + ' sh=' + el.scrollHeight
        + ' pos=' + cs.position);
    });
    document.title = 'PROBE:' + JSON.stringify(out);
  }, 1200);
});
</script>
"""


def find_edge():
    for p in (r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
              r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"):
        if os.path.exists(p):
            return p
    raise SystemExit("找不到 Edge")


def main(w="1440", h="900"):
    tmp_dir = tempfile.mkdtemp(prefix="szu-probe-")
    try:
        # 页面必须和 assets/ 同级才找得到图，所以整个 desktop 拷一份太慢，
        # 只复制页面，并在 desktop 下临时放一个注入版
        probe_path = os.path.join(DESKTOP, "_probe.html")
        page = open(SRC, encoding="utf-8").read()
        open(probe_path, "w", encoding="utf-8", newline="").write(
            page.replace("</body>", PROBE + "</body>"))

        url = "file:///" + probe_path.replace("\\", "/")
        r = subprocess.run([find_edge(), "--headless=new", "--disable-gpu",
                            "--window-size=%s,%s" % (w, h),
                            "--virtual-time-budget=9000",
                            "--user-data-dir=" + os.path.join(tmp_dir, "ud"),
                            "--dump-dom", url],
                           capture_output=True, timeout=180)
        dom = r.stdout.decode("utf-8", "ignore")
        m = re.search(r"<title>PROBE:(.*?)</title>", dom, re.S)
        if not m:
            print("没抓到探针结果。前 800 字节的 DOM：")
            print(dom[:800])
            return 1
        raw = m.group(1)
        raw = (raw.replace("&quot;", '"').replace("&amp;", "&")
                  .replace("&lt;", "<").replace("&gt;", ">"))
        d = json.loads(raw)
        print("视口      : %s" % d["viewport"])
        print("文档总高  : %d" % d["docScrollH"])
        print("body 高度 : %d" % d["bodyH"])
        over = d["docScrollH"] - int(d["viewport"].split("x")[1])
        print("超出视口  : %d px  %s" % (over, "（要滚动）" if over > 0 else "（不用滚）"))
        print("\nbody 直接子元素：")
        for b in d["blocks"]:
            print("   " + b)
        print("\n关键选择器：")
        for s in d["sel"]:
            print("   " + s)
        if d.get("ovf"):
            print("\n溢出的块（内容比可见高，可能被裁掉）：")
            for o in d["ovf"]:
                print("   " + o)
        else:
            print("\n溢出的块：无")
        return 0
    finally:
        p = os.path.join(DESKTOP, "_probe.html")
        if os.path.exists(p):
            os.remove(p)
        shutil.rmtree(tmp_dir, ignore_errors=True)


if __name__ == "__main__":
    a = sys.argv[1:]
    sys.exit(main(*(a[:2])))

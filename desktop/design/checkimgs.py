"""在本地页面副本上注入脚本，等图片真正 settle 之后再读加载结果。

和 probe_live.py 的区别：这个不连服务，直接读 desktop/index.html 主副本 ——
因为它和 assets/ 在同一层，相对路径 assets/art/... 用 file:// 打开是找得到的
（注意：assets/index.html 那份也能开，两者同层，都行）。

关键改进：virtual-time-budget 拉到 12 秒，并且注入的脚本自己轮询等待，
不再用固定 setTimeout —— 之前的 1.2 秒在无头模式下可能根本没等到解码。

用法: python checkimgs.py [width] [height]
"""
import os
import re
import subprocess
import sys

PAGEDIR = r"D:\szuNet\desktop"
PAGE = os.path.join(PAGEDIR, "index.html")


def find_edge():
    for p in (r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
              r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"):
        if os.path.exists(p):
            return p
    raise SystemExit("找不到 Edge")


JS = """
<script>
(function(){
  var tries = 0, MAX = 60;
  function collect(){
    var imgs = document.querySelectorAll("img");
    var bad = [], ok = 0;
    for (var i = 0; i < imgs.length; i++) {
      var im = imgs[i];
      var src = im.getAttribute("src") || "?";
      if (im.naturalWidth > 0) { ok++; }
      else {
        bad.push(src + "[c=" + im.complete + ",n=" + im.naturalWidth + "]");
      }
    }
    return { ok: ok, bad: bad, total: imgs.length };
  }
  function tick(){
    tries++;
    var r = collect();
    // 全都好了，或者等够次数，就写结果
    if (r.bad.length === 0 || tries >= MAX) {
      document.title = "IMGCHK|ok=" + r.ok + "/" + r.total
        + " bad=" + r.bad.length + " || " + r.bad.join(" | ");
      return;
    }
    setTimeout(tick, 200);
  }
  // load 事件之后再开始轮询；某些情况下 load 早于图片解码完成
  if (document.readyState === "complete") { setTimeout(tick, 100); }
  else { window.addEventListener("load", function(){ setTimeout(tick, 100); }); }
})();
</script>
"""


def run(w="1280", h="900", scale="1"):
    src = open(PAGE, encoding="utf-8").read()
    if "</body>" not in src:
        raise SystemExit("页面里没有 </body>，没法注入")
    src = src.replace("</body>", JS + "</body>")
    tmp = os.path.join(PAGEDIR, "_checkimgs.html")
    open(tmp, "w", encoding="utf-8").write(src)
    try:
        cmd = [
            find_edge(),
            "--headless=new", "--disable-gpu", "--hide-scrollbars",
            "--force-device-scale-factor=" + scale,
            "--window-size=%s,%s" % (w, h),
            "--user-data-dir=" + os.path.join(os.environ["TEMP"], "edge-szu-checkimgs"),
            "--virtual-time-budget=12000", "--dump-dom",
            "file:///" + tmp.replace("\\", "/"),
        ]
        r = subprocess.run(cmd, capture_output=True, timeout=180)
        html = r.stdout.decode("utf-8", "ignore")
        m = re.search(r"<title>IMGCHK\|(.*?)</title>", html, re.S)
        if not m:
            print("没读到结果。DOM 前 400 字：")
            print(html[:400])
            return
        body = m.group(1)
        head, _, bad = body.partition(" || ")
        print("视口 %sx%s 倍率 %s" % (w, h, scale))
        print("  " + head.strip())
        bad = bad.strip()
        if bad:
            for item in bad.split(" | "):
                print("    [坏] " + item.strip())
        else:
            print("    没有坏图")
    finally:
        try:
            os.remove(tmp)
        except OSError:
            pass


if __name__ == "__main__":
    a = sys.argv[1:]
    run(*(a or ["1280", "900", "1"]))

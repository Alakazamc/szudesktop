"""量页面上某个元素的位置和尺寸。

用法: python probe.py ".avatar" 选择器 [选择器2 ...]

做法：往页面里塞一段脚本，把元素的位置写进 document.title，
再用 Edge 无头 dump-dom 把 title 读出来。

为什么不用 JS 直接打印：无头模式没有控制台回传，dump-dom 才拿得到东西。
"""
import os
import re
import subprocess
import sys

PAGEDIR = r"D:\szudesktop\desktop"
PAGE = os.path.join(PAGEDIR, "index.html")


def find_edge():
    for p in (r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
              r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"):
        if os.path.exists(p):
            return p
    raise SystemExit("找不到 Edge")


def probe(selectors):
    src = open(PAGE, encoding="utf-8").read()
    # 把原来</body>前的内容保留，只追加测量脚本
    js = """
<script>
(function(){
  var out=[];
  var sels=%s;
  sels.forEach(function(s){
    var ns=document.querySelectorAll(s);
    if(!ns.length){ out.push(s+" :: 没找到"); return; }
    ns.forEach(function(n,i){
      var r=n.getBoundingClientRect();
      out.push(s+"["+i+"] x="+Math.round(r.left)+" y="+Math.round(r.top+window.scrollY)
        +" w="+Math.round(r.width)+" h="+Math.round(r.height));
    });
  });
  out.push("PAGEW="+document.documentElement.scrollWidth+" PAGEH="+document.body.scrollHeight);
  document.title="PROBE|"+out.join(" ;; ");
})();
</script>
""" % str(selectors).replace("'", '"')
    src = src.replace("</body>", js + "</body>")
    tmp = os.path.join(PAGEDIR, "_probe.html")
    open(tmp, "w", encoding="utf-8").write(src)

    edge = find_edge()
    cmd = [edge, "--headless=new", "--disable-gpu", "--window-size=1280,900",
           "--user-data-dir=" + os.path.join(os.environ["TEMP"], "edge-szu-probe"),
           "--virtual-time-budget=3000", "--dump-dom",
           "file:///" + tmp.replace("\\", "/")]
    r = subprocess.run(cmd, capture_output=True)
    os.remove(tmp)
    html = r.stdout.decode("utf-8", "ignore")
    m = re.search(r"<title>PROBE\|(.*?)</title>", html, re.S)
    if not m:
        print("没读到测量结果。DOM 前 500 字：")
        print(html[:500])
        return
    for part in m.group(1).split(" ;; "):
        print("  " + part.strip())


if __name__ == "__main__":
    probe(sys.argv[1:])

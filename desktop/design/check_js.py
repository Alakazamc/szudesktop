"""检查 index.html 里的 JS：语法过不过、有没有「调用了但根本没定义」的函数。

为什么要有这个脚本：登录页那次翻车就是「refreshLogin() 被调用但从来没定义」。
这种错在浏览器里只抛一条 ReferenceError，页面照样显示，按钮点了没反应而已 ——
不打开控制台根本发现不了，冒烟测试也查不出来（它只比对字节数和几个标记）。

跑法：
    python desktop/design/check_js.py
返回码非 0 表示有问题。
"""
import os
import re
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))  # desktop/
PAGE = os.path.join(ROOT, "index.html")
NODE = r"C:\Users\Alakazam\.workbuddy\binaries\node\versions\22.22.2-3\node.exe"

# 可以传个路径进来检查别的副本（自测时用得上）
if len(sys.argv) > 1:
    PAGE = sys.argv[1]

html = open(PAGE, encoding="utf-8").read()

# 1) 抽出所有内联 script（跳过带 src 的外链）
blocks = re.findall(r"<script(?![^>]*\bsrc=)[^>]*>(.*?)</script>", html, re.S)
if not blocks:
    print("!! 没找到内联 script，检查脚本本身是不是过时了")
    sys.exit(1)
js = "\n;\n".join(blocks)
print("抽出 %d 段脚本，共 %d 字符" % (len(blocks), len(js)))

fails = []

# 2) 语法检查交给 node
if os.path.exists(NODE):
    tmp = os.path.join(tempfile.gettempdir(), "_szu_page_check.js")
    with open(tmp, "w", encoding="utf-8") as f:
        f.write(js)
    r = subprocess.run([NODE, "--check", tmp], capture_output=True, text=True)
    if r.returncode == 0:
        print("[OK] JS 语法通过")
    else:
        # node 报的行号是拼接后的，和 index.html 不完全对应，但足够定位
        print("[XX] JS 语法错误:\n" + (r.stderr or r.stdout))
        fails.append("语法错误")
    os.remove(tmp)
else:
    print("[--] 找不到 node，跳过语法检查")

# 3) 「调用了但没定义」检查
#    只管页面自己定义的函数，浏览器内置的和 DOM 上的方法一概不碰。
defined = set(re.findall(r"\bfunction\s+([A-Za-z_$][\w$]*)", js))
defined |= set(re.findall(r"\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(", js))
defined |= set(re.findall(r"\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?function", js))
defined |= set(re.findall(r"\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*[A-Za-z_$][\w$]*\s*=>", js))
# 普通变量也收进来：像 pagerTicks 这种拿来 forEach 的不算未定义
defined |= set(re.findall(r"\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)", js))

# 函数参数也算「定义过」。
# 少了这一条会误报：pagerTicks.forEach(f => f()) 里的 f 是形参，
# 但它长得就像「调用了一个没定义的 f」。
defined |= set(re.findall(r"(?<![\w$.])([A-Za-z_$][\w$]*)\s*=>", js))          # x => ...
for params in re.findall(r"\(([^()]*)\)\s*=>", js):                             # (a, b) => ...
    defined |= {p.strip() for p in params.split(",") if re.fullmatch(r"[A-Za-z_$][\w$]*", p.strip())}
for params in re.findall(r"\bfunction\s*[A-Za-z_$\w]*\s*\(([^()]*)\)", js):     # function f(a, b)
    defined |= {p.strip() for p in params.split(",") if re.fullmatch(r"[A-Za-z_$][\w$]*", p.strip())}

# 语言关键字、内置对象、DOM 方法：全都不参与判断
IGNORE = {
    "if", "for", "while", "switch", "catch", "return", "typeof", "new", "delete",
    "function", "await", "async", "in", "of", "do", "else", "try", "finally",
    "throw", "void", "yield", "case", "break", "continue", "class", "extends",
    "super", "this", "null", "true", "false", "undefined",
    "setTimeout", "setInterval", "clearInterval", "clearTimeout", "fetch",
    "requestAnimationFrame", "cancelAnimationFrame", "getComputedStyle",
    "addEventListener", "removeEventListener", "encodeURIComponent",
    "decodeURIComponent", "structuredClone", "queueMicrotask",
    "parseInt", "parseFloat", "isNaN", "String", "Number", "Boolean", "Array",
    "Object", "JSON", "Math", "Date", "Error", "Promise", "Set", "Map", "RegExp",
    "console", "document", "window", "localStorage", "location", "alert",
    "getElementById", "querySelector", "querySelectorAll", "createElement",
    "appendChild", "append", "remove", "forEach", "map", "filter", "reduce",
    "push", "slice", "splice", "join", "split", "replace", "trim", "toString",
    "padStart", "repeat", "includes", "indexOf", "exec", "test", "match",
    "add", "toggle", "contains", "assign", "stringify", "parse", "keys",
    "values", "entries", "min", "max", "round", "floor", "ceil", "abs", "random",
    "now", "sort", "find", "some", "every", "concat", "reverse", "startsWith",
    "endsWith", "toLowerCase", "toUpperCase", "charAt", "charCodeAt", "close",
    "write", "open", "then", "catch", "finally", "bind", "call", "apply",
    "getMonth", "getDate", "getDay", "getHours", "getMinutes", "getTime",
    "toDateString", "setDeadline", "focus", "blur", "click", "preventDefault",
    "insertBefore", "removeChild", "setAttribute", "getAttribute", "scrollBy",
    "closest", "getItem", "setItem", "hasOwnProperty", "isArray", "from",
}

# 收集「像函数调用」的名字：foo( 但前面不是点号（那是方法调用）
called = set()
for m in re.finditer(r"(?<![.\w$])([A-Za-z_$][\w$]*)\s*\(", js):
    called.add(m.group(1))

missing = sorted(n for n in called - defined - IGNORE)
if missing:
    print("[XX] 调用了但没有定义的函数 %d 个:" % len(missing))
    for n in missing:
        # 顺手把第一次出现的位置报出来，方便定位
        i = js.find(n + "(")
        line = js[:i].count("\n") + 1 if i >= 0 else 0
        print("     %-24s 首次出现在拼接脚本第 %d 行" % (n, line))
    fails.append("有未定义函数")
else:
    print("[OK] 没有「调用了但没定义」的函数（共 %d 个自定义名字）" % len(defined))

# 4) 页面上带 id 的按钮/输入框/下拉框，有没有被 JS 引用
#    只挑交互控件（button/input/select），纯展示的 div 不管。
ctrl_ids = set()
for m in re.finditer(r"<(button|input|select)\b[^>]*\bid=\"([^\"]+)\"", html):
    ctrl_ids.add(m.group(2))
unused = sorted(i for i in ctrl_ids if i not in js)
if unused:
    print("[!!] 这些控件有 id 但 JS 里一次都没提到（可能是忘接事件了）:")
    for i in unused:
        print("     " + i)
    fails.append("有控件没接上")
else:
    print("[OK] 所有 button/input/select 控件都在 JS 里被引用了（共 %d 个）" % len(ctrl_ids))

print()
if fails:
    print("结果: 不通过 —— " + "、".join(fails))
    sys.exit(1)
print("结果: 全部通过")

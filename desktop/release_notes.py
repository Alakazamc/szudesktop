"""从 CHANGELOG.md 抽取某个版本的发布说明。

用法:
    python desktop/release_notes.py beta0.8              # 打印到标准输出
    python desktop/release_notes.py beta0.8 -o notes.md  # 写文件（CI 用）

CHANGELOG.md 是 GitHub Release 正文的唯一来源。抽不到、正文是空的、
或者正文里还留着占位符，都以非零退出码失败——CI 的 release job 会在上传
附件之前停下来。beta0.7 那次发布说明只有一行 compare 链接，就是因为
当时没有任何关卡拦着。
"""
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CHANGELOG = os.path.join(ROOT, "CHANGELOG.md")

UNRELEASED = "未发布"

# 附件名与 CI 实际上传的一致（release.yml 的 build-cli 矩阵 + make_release.py）。
# 独立 EXE 不带版本号、也没有单独的 .sha256，只有 ZIP 有——按 beta0.7.1 的
# 真实附件列表核对过，别凭印象写。
DOWNLOADS = """### 下载

- `szudesktop-__VERSION__-windows-amd64.zip` — 解压即用，旁边是同名 `.sha256` 校验文件
- `szudesktop-windows-amd64.exe` — 单文件版，与 ZIP 里的程序逐字节一致
- 命令行版：`szunet-windows-amd64.exe`、`szunet-darwin-amd64`、`szunet-darwin-arm64`、`szunet-linux-amd64`、`szunet-linux-arm64`
"""


class NotesError(Exception):
    """发布说明不可用。错误信息直接给发版的人看，要写清该改什么。"""


def extract(text, version):
    """返回 CHANGELOG.md 里 `## <version>` 那一节的正文（不含标题）。"""
    if version == UNRELEASED:
        raise NotesError(
            "「%s」不是版本号：发版前请把 CHANGELOG.md 里那一节的标题改成真实版本号" % UNRELEASED)

    heading = "## " + version
    lines = text.splitlines()
    start = None
    for i, line in enumerate(lines):
        # 只接受完全相等的标题，否则 beta0.7 会命中 beta0.7.1
        if line.strip() == heading:
            start = i + 1
            break
    if start is None:
        raise NotesError(
            "CHANGELOG.md 里没有 `## %s` 这一节。发版前必须先写好这个版本的更新说明" % version)

    body = []
    for line in lines[start:]:
        if line.startswith("## "):
            break
        body.append(line)
    text_body = "\n".join(body).strip()

    if not text_body:
        raise NotesError("CHANGELOG.md 的 `## %s` 一节是空的，不能发一个没有说明的版本" % version)
    if "__VERSION__" in text_body:
        raise NotesError("CHANGELOG.md 的 `## %s` 一节里还留着 __VERSION__ 占位符" % version)
    return text_body


def render(text, version):
    """正文 + 下载清单。清单由版本号生成，所以 CHANGELOG 里不要自己写。"""
    return extract(text, version) + "\n\n---\n\n" + DOWNLOADS.replace("__VERSION__", version)


def load(path=CHANGELOG):
    with open(path, encoding="utf-8") as f:
        return f.read()


def main(argv):
    if len(argv) < 2:
        print("用法: python desktop/release_notes.py <版本号> [-o 输出文件]", file=sys.stderr)
        return 2
    version = argv[1]
    out = argv[argv.index("-o") + 1] if "-o" in argv else None

    try:
        body = render(load(), version)
    except OSError as e:
        print("!! 读不到 %s: %s" % (CHANGELOG, e), file=sys.stderr)
        return 1
    except NotesError as e:
        print("!! " + str(e), file=sys.stderr)
        return 1

    if out:
        # newline="\n"：CI 的 windows 任务会用文本模式写文件，默认把 \n 翻成 \r\n
        with open(out, "w", encoding="utf-8", newline="\n") as f:
            f.write(body + "\n")
        print("-> %s（%d 字节）" % (out, len(body.encode("utf-8"))))
    else:
        sys.stdout.write(body + "\n")
    return 0


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")
    sys.exit(main(sys.argv))

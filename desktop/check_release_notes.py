"""发布说明抽取的回归检查：python desktop/check_release_notes.py

CHANGELOG.md 是 GitHub Release 正文的唯一来源。抽取逻辑一旦出错，
发出去的说明就会是空的、或者带上别的版本的内容——beta0.7 那次
只发了一行 compare 链接，就是因为没有这道关卡。
"""
import sys

# 与 make_release.py / smoke_windows.py 一致：GitHub 的 Windows runner 可能是 CP1252，
# 中文检查名统一按 UTF-8 输出。
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

import release_notes  # noqa: E402

SAMPLE = """# 更新日志

格式约定：每节以 `## <版本号>` 开头。

## 未发布

- 还没发的东西

## beta0.7.1

修复版：只含一个安全修复。

### 相对 beta0.7 的变化

- macOS 保存凭据不再经命令行参数暴露

## beta0.7

- 设置页可开关开机自启
"""

count = 0


def check(name, fn):
    global count
    fn()
    count += 1
    print("PASS " + name)


def expect_error(fn, why):
    """必须抛 NotesError，而且错误信息不能是空的——用户得看得懂该改什么。"""
    try:
        fn()
    except release_notes.NotesError as e:
        if not str(e).strip():
            raise AssertionError(why + "：错误信息是空的")
        return
    raise AssertionError(why + "：居然没报错")


def test_extracts_section():
    body = release_notes.extract(SAMPLE, "beta0.7.1")
    assert "修复版：只含一个安全修复。" in body, "正文没抽到"
    assert "### 相对 beta0.7 的变化" in body, "小节标题被吃掉了"


def test_stops_at_next_version():
    body = release_notes.extract(SAMPLE, "beta0.7.1")
    assert "设置页可开关开机自启" not in body, "串到下一节去了"


def test_excludes_heading_itself():
    body = release_notes.extract(SAMPLE, "beta0.7")
    assert not body.lstrip().startswith("##"), "标题行被当成正文了"


def test_version_must_match_exactly():
    body = release_notes.extract(SAMPLE, "beta0.7")
    assert "macOS" not in body, "beta0.7 命中了 beta0.7.1 的内容"


def test_render_fills_download_list():
    out = release_notes.render(SAMPLE, "beta0.7.1")
    assert "szudesktop-beta0.7.1-windows-amd64.zip" in out, "桌面 ZIP 名字不对"
    assert "szunet-darwin-arm64" in out, "命令行附件缺了"
    assert "__VERSION__" not in out, "占位符没替换"


def test_render_keeps_body_before_download_list():
    out = release_notes.render(SAMPLE, "beta0.7.1")
    assert out.index("修复版") < out.index("szudesktop-beta0.7.1-windows-amd64.zip"), "顺序不对"


def test_installer_download_versions():
    current = release_notes.render("## beta0.8.0\n\n- Electron 安装版\n", "beta0.8.0")
    assert "szuDesktop-Setup-0.8.0.exe" in current, "安装包名不符合 semver 命名"
    assert "szudesktop-beta0.8.0-windows-amd64.zip" in current, "旧版便携包必须保留"
    assert ".sha256" in current, "缺少安装包校验说明"
    assert "__SEMVER__" not in current, "安装包版本占位符没有替换"
    assert "szuDesktop-Setup" not in release_notes.render(SAMPLE, "beta0.7.1"), "旧版本不应凭空多出安装包"


check("抽取到对应版本那一节的正文", test_extracts_section)
check("在下一个版本标题处停下，不带进 beta0.7 的内容", test_stops_at_next_version)
check("返回的正文不含标题行本身", test_excludes_heading_itself)
check("版本号必须精确匹配，beta0.7 不会命中 beta0.7.1", test_version_must_match_exactly)
check("render 把版本号填进下载清单", test_render_fills_download_list)
check("render 保留正文，再追加下载清单", test_render_keeps_body_before_download_list)
check("新版本包含安装包，旧版本不虚构附件", test_installer_download_versions)

check("CHANGELOG 里没有这个版本时报错",
      lambda: expect_error(lambda: release_notes.extract(SAMPLE, "beta0.8"), "缺版本"))
check("只有标题、正文是空的也报错",
      lambda: expect_error(lambda: release_notes.extract("## beta0.9\n\n## beta0.8\n\n正文\n", "beta0.9"), "空正文"))
check("正文只有空白字符同样报错",
      lambda: expect_error(lambda: release_notes.extract("## beta0.9\n   \n\n## beta0.8\n\n正文\n", "beta0.9"), "空白正文"))
check("不能拿「未发布」当版本号发布",
      lambda: expect_error(lambda: release_notes.extract(SAMPLE, "未发布"), "未发布"))
check("正文里留着未替换的占位符时报错",
      lambda: expect_error(lambda: release_notes.extract("## beta0.9\n\n- 修复了 __VERSION__ 的问题\n", "beta0.9"), "占位符"))

print("%d release-notes checks passed" % count)

"""给编好的 Windows exe 塞进图标和版本信息（纯 Python，不装任何工具）。

为什么不走常规路子：
  Go 自己**不支持**往 Windows exe 里加资源，社区做法是装
  `goversioninfo` + `windres`（或 `rsrc`）。但这台机器上没有 windres，
  而装它意味着给项目加一个构建期依赖 —— 这个项目当初就是冲着
  "克隆下来 go build 就能出三端" 设计的，加依赖不合适。
所以这里直接改 PE 文件：解析段表 → 追加一个 .rsrc 段 → 挂到
可选头的资源目录项上。改动量小、可逆、零依赖。

做的事：
  1. 读 exe，校验 PE 结构
  2. 构造资源目录：图标组(14) + 图标(3) + 版本信息(16)
  3. 追加一个 .rsrc 段装这些数据，修正 section 数、SizeOfImage、
     NumberOfRvaAndSizes 和 data directory[2]
  4. 校验：重新解析一遍，确认资源目录能读回来

用法:
    python add_resource.py <exe路径> [--ico 图标路径] [--version 0.2.0]

注意：处理过的 exe 用 `go build` 重新编译会覆盖掉，所以这一步必须跟在
编译之后（build-windows.py 里已经接上了）。
"""
import os
import re
import struct
import sys

# Windows 资源类型编号
RT_ICON = 3
RT_GROUP_ICON = 14
RT_VERSION = 16


# ---------------------------------------------------------------- PE 读写
class PE:
    def __init__(self, data):
        self.d = bytearray(data)
        if self.d[:2] != b"MZ":
            raise ValueError("不是 PE 文件（没有 MZ 头）")
        self.pe_off = struct.unpack_from("<I", self.d, 0x3C)[0]
        if self.d[self.pe_off:self.pe_off + 4] != b"PE\x00\x00":
            raise ValueError("PE 签名不对")
        self.nsec = struct.unpack_from("<H", self.d, self.pe_off + 6)[0]
        self.opt_size = struct.unpack_from("<H", self.d, self.pe_off + 20)[0]
        self.opt_off = self.pe_off + 24
        magic = struct.unpack_from("<H", self.d, self.opt_off)[0]
        if magic != 0x20B:
            raise ValueError("只支持 PE32+（64 位），拿到 %#x" % magic)
        self.sec_off = self.opt_off + self.opt_size

    def sections(self):
        out = []
        for i in range(self.nsec):
            e = self.sec_off + i * 40
            name = self.d[e:e + 8].rstrip(b"\x00").decode("latin1")
            vsize, vaddr, rawsize, rawptr = struct.unpack_from("<IIII", self.d, e + 8)
            chars = struct.unpack_from("<I", self.d, e + 36)[0]
            out.append(dict(idx=i, off=e, name=name, vsize=vsize, vaddr=vaddr,
                            rawsize=rawsize, rawptr=rawptr, chars=chars))
        return out

    def data_dirs(self):
        n = struct.unpack_from("<I", self.d, self.opt_off + 108)[0]
        base = self.opt_off + 112
        return n, base

    def align_up(self, v, a):
        return (v + a - 1) // a * a

    def section_align(self):
        return struct.unpack_from("<I", self.d, self.opt_off + 32)[0]

    def file_align(self):
        return struct.unpack_from("<I", self.d, self.opt_off + 36)[0]


# ---------------------------------------------------------------- 资源构造
def make_res_dir(entries):
    """entries: [(type_id, name_id, data_bytes), ...]

    返回 (资源目录字节, RVA修正函数)。
    资源目录是三层树：类型 → 名字 → 语言，每层一个目录表。
    结构简单（每类只有一项），所以直接拼出来，不做通用树。
    """
    # 每类资源的叶子数据放在 0x1000 对齐的位置后面
    # 先算目录部分大小：1 个根 + 3 个类型子目录，每个目录 16+8n 字节
    # 目录部分的布局（全部相对资源段起点）：
    #   根目录   16 + 8*N
    #   类型目录 每类 16 + 8*1
    #   名字目录 每类 16 + 8*1
    #   数据项表 每类 16
    #   数据块   ...
    ROOT_OFF = 0
    root_n = len(entries)
    root_size = 16 + 8 * root_n
    type_off = root_size
    type_size = 16 + 8 * 1          # 每个类型子目录只有 1 项（名字层）
    lang_size = 16 + 8 * 1          # 每个名字子目录只有 1 项（语言层）
    names_off = type_off + type_size * root_n
    data_tbl_off = names_off + lang_size * root_n
    dir_total = data_tbl_off + 16 * root_n

    # 数据区从 4 字节对齐处开始
    DATA_OFF = (dir_total + 3) // 4 * 4

    root = bytearray()
    type_dirs = bytearray()
    lang_dirs = bytearray()
    data_entries = bytearray()
    blobs = bytearray()

    for i, (tid, nid, payload) in enumerate(entries):
        t_off = type_off + type_size * i
        l_off = names_off + lang_size * i
        d_off = DATA_OFF + len(blobs)

        # 根项：类型 ID → 指向该类型的子目录（高位 1 表示"是目录"）
        root += struct.pack("<II", tid, 0x80000000 | t_off)
        # 类型目录：1 个 ID 项，名字 ID → 指向名字子目录（同样 16 字节头）
        type_dirs += struct.pack("<IIHHHH", 0, 0, 0, 0, 0, 1)
        type_dirs += struct.pack("<II", nid, 0x80000000 | l_off)
        # 名字目录：1 个 ID 项，语言 0x409 → 指向数据项
        lang_dirs += struct.pack("<IIHHHH", 0, 0, 0, 0, 0, 1)
        lang_dirs += struct.pack("<II", 0x409, d_off)          # 高位 0 表示"是叶子"

        # 数据项：RVA(占位，外面回填) + Size + CodePage + Reserved
        data_entries += struct.pack("<IIII", 0, len(payload), 0, 0)
        blobs += payload
        while len(blobs) % 4:
            blobs += b"\x00"

    # 拼起来（数据项里的 RVA 由调用方回填）
    #
    # ⚠️ IMAGE_RESOURCE_DIRECTORY 是 **16** 字节：
    #     Characteristics(4) + TimeDateStamp(4) + MajorVersion(2) + MinorVersion(2)
    #     + NumberOfNamedEntries(2) + NumberOfIdEntries(2)
    # 少写 4 字节的话，整个树会错位 4 —— 外部看就是"资源目录里全是垃圾 ID"。
    head = bytearray()
    head += struct.pack("<IIHHHH", 0, 0, 0, 0, 0, root_n)   # 根目录 header
    head += root
    head += type_dirs
    head += lang_dirs
    # 数据项表紧跟在名字目录之后。**这里必须用组装后的实际长度算**，
    # 不能拿预计的 names_off 去推 —— type_dirs / lang_dirs 是两个独立的
    # 累加器，文件里的真实顺序是"根 → 所有类型目录 → 所有名字目录 → 数据项表"，
    # 用公式推出来的偏移会跟实际差一截，回填 RVA 时就写错位置：
    # 表现为资源里的图标 RVA 变成 ASCII "\x89PNG"（等于把 PNG 文件头当成了 RVA），
    # 而结构检查还能过，只有真去看数据才发现。
    data_tbl_actual = len(head)
    assert data_tbl_actual == data_tbl_off, (data_tbl_actual, data_tbl_off)
    head += data_entries
    assert DATA_OFF >= len(head), ("目录算小了", DATA_OFF, len(head))
    head += b"\x00" * (DATA_OFF - len(head))
    head += blobs
    # 各资源数据块在段内的偏移（用来回填 RVA）
    offsets = [DATA_OFF + sum(len(p) + (-len(p)) % 4 for _, _, p in entries[:i])
               for i in range(len(entries))]
    return bytes(head), offsets, data_tbl_off


def bmp_and_icon(ico_path):
    """拆开 .ico，返回 [(原始PNG/BMP数据, 宽, 高, 位深), ...] 和图标组数据。

    ICO 里嵌的如果是 PNG（Vista 之后都这样），资源里可以直接原样放。
    """
    d = open(ico_path, "rb").read()
    reserved, itype, count = struct.unpack_from("<HHH", d, 0)
    assert reserved == 0 and itype == 1, "不是 ICO"
    items = []
    for i in range(count):
        w, h, colors, res, planes, bpp, size, off = struct.unpack_from("<BBBBHHII", d, 6 + i * 16)
        items.append(dict(w=w or 256, h=h or 256, bpp=bpp or 32,
                          data=d[off:off + size]))
    # 图标组（GRPICONDIR）：头 + 每项 14 字节
    grp = struct.pack("<HHH", 0, 1, len(items))
    for i, it in enumerate(items):
        w = 0 if it["w"] >= 256 else it["w"]
        h = 0 if it["h"] >= 256 else it["h"]
        grp += struct.pack("<BBBBHHIH", w, h, 0, 0, 1, it["bpp"], len(it["data"]), i + 1)
    return items, grp


def utf16z(s):
    return s.encode("utf-16-le") + b"\x00\x00"


def pad4(b):
    while len(b) % 4:
        b += b"\x00"
    return b


def version_info(ver, exe_name):
    """构造 VS_VERSION_INFO 资源（块结构，每个块自带长度）。"""
    def block(key, value, is_text):
        if is_text:
            payload = utf16z(value)
            val_len = len(payload) // 2
            val_field = struct.pack("<HH", len(payload), val_len) + payload
        else:
            val_field = struct.pack("<HH", len(value), 0) + value
            val_field = pad4(val_field)
        body = struct.pack("<HH", 0, 0)          # wLength 占位, wValueLength
        body = struct.pack("<HHH", 0, len(val_field), 1) + utf16z(key) + val_field
        body = pad4(body)
        return struct.pack("<H", len(body)) + body[2:]

    # Windows 固定版本字段只能放四段数字；展示文字仍保留 beta0.1 这类标签。
    # beta0.1 -> 0.1.0.0，0.1.0-beta.1 -> 0.1.0.1。
    parts = [int(x) for x in re.findall(r"\d+", ver)]
    if not parts:
        parts = [0]
    while len(parts) < 4:
        parts.append(0)
    ms, mn, bld, rev = parts[:4]
    # 版本号打包成两个 DWORD
    ms_hex = (ms << 16) | mn
    ls_hex = (bld << 16) | rev

    fixed = struct.pack("<IIIIIIIIIIIIII",
                        ms_hex, ls_hex, 0, 0,
                        0x3F, 0, 0x40004, 1,
                        ms_hex, ls_hex, 0x3F, 0, 0x40004, 1)

    sfi = block("StringFileInfo", b"", False)
    # 单个 StringTable（040904B0 = 英文/Unicode）
    st = struct.pack("<HHH", 0, 0, 1) + utf16z("040904B0")
    for k, v in [("CompanyName", "SZUNet"),
                 ("FileDescription", "szuDesktop 深大校园服务台"),
                 ("FileVersion", ver),
                 ("InternalName", exe_name),
                 ("OriginalFilename", exe_name + ".exe"),
                 ("ProductName", "szuDesktop"),
                 ("ProductVersion", ver),
                 ("LegalCopyright", "MIT License")]:
        st += block(k, v, True)
    st = pad4(st)
    st = struct.pack("<H", len(st)) + st[2:]
    sfi = block("StringFileInfo", st, False)

    vt = block("Translation", struct.pack("<HH", 0x409, 1200), False)
    vs = block("VarFileInfo", vt, False)

    root = struct.pack("<HHH", 0, 0, 0) + utf16z("VS_VERSION_INFO") \
        + struct.pack("<H", 0) + pad4(fixed) + sfi + vs
    root = pad4(root)
    root = struct.pack("<H", len(root)) + root[2:]
    return root


# ---------------------------------------------------------------- 主流程
def add_resources(exe_path, ico_path, ver, exe_name):
    pe = PE(open(exe_path, "rb").read())
    sects = pe.sections()
    if any(s["name"] == ".rsrc" for s in sects):
        print("   已经有 .rsrc 段了，跳过（先 go build 重新生成再跑本脚本）")
        return False

    items, grp = bmp_and_icon(ico_path)
    entries = [(RT_GROUP_ICON, 1, grp)]
    for i, it in enumerate(items):
        entries.append((RT_ICON, i + 1, it["data"]))
    entries.append((RT_VERSION, 1, version_info(ver, exe_name)))

    res_data, offsets, data_tbl_off = make_res_dir(entries)

    sec_align = pe.section_align()
    file_align = pe.file_align()

    # ------------------------------------------------------------
    # 段放哪：**占用 .symtab 的位置**，而不是在末尾追加。
    #
    # 试过追加到最后一个段后面，结果 exe 直接跑不起来：
    #   OSError: [WinError 193] %1 不是有效的 Win32 应用程序。
    # 原因是 Go 链接器会留一个空的 .symtab 段（只有 4 字节 +
    # 零填充，`-s -w` 也没去掉），而 Windows 加载器要求段按
    # VirtualAddress 升序排列、并且不认这种排在 .rsrc 前面的表。
    # 追加出来的段顺序变成 [... .reloc, .symtab, .rsrc]，加载器就拒了。
    #
    # .symtab 里没有任何运行时需要的东西（就是个空符号表），
    # 直接拿它的位置来用：段数不变、顺序不乱、也不用挪别的段。
    # ------------------------------------------------------------
    victim = None
    for s in sects:
        if s["name"] == ".symtab" and s["vsize"] <= 8:
            victim = s
            break

    if victim is not None:
        new_vaddr = victim["vaddr"]
        new_rawptr = victim["rawptr"]
        old_rawsize = victim["rawsize"]
        slot_off = victim["off"]
        print("   复用空的 .symtab 段（vaddr %#x，原大小 %d）" % (new_vaddr, old_rawsize))
    else:
        # 没有可复用的就追加到最后（有些构建配置下确实没有 .symtab）
        last = sects[-1]
        new_vaddr = pe.align_up(last["vaddr"] + last["vsize"], sec_align)
        new_rawptr = pe.align_up(last["rawptr"] + last["rawsize"], file_align)
        old_rawsize = 0
        slot_off = None
        print("   没有可复用的段，追加新段（vaddr %#x）" % new_vaddr)

    new_vsize = len(res_data)
    new_rawsize = pe.align_up(new_vsize, file_align)

    # 写段头
    hdr = bytearray()
    hdr += b".rsrc".ljust(8, b"\x00")                       # Name            8
    hdr += struct.pack("<IIII", new_vsize, new_vaddr,       # VirtualSize     4
                       new_rawsize, new_rawptr)             # VirtualAddress  4
                                                            # SizeOfRawData   4
                                                            # PtrToRawData    4
    hdr += struct.pack("<II", 0, 0)                         # Reloc/Lineno    8
    hdr += struct.pack("<HH", 0, 0)                         # NumReloc/NumLine 4
    hdr += struct.pack("<I", 0x40000040)                    # 已初始化数据 | 可读
    assert len(hdr) == 40, len(hdr)

    if slot_off is not None:
        pe.d[slot_off:slot_off + 40] = hdr
    else:
        room = None
        sec_table_end = pe.sec_off + pe.nsec * 40
        for probe in range(0, 40 * 4, 8):
            chunk = pe.d[sec_table_end + probe: sec_table_end + probe + 40]
            if len(chunk) < 40 or all(b == 0 for b in chunk):
                room = sec_table_end + probe
                break
        if room is None:
            raise RuntimeError("段表后面没有空位放新段头")
        pe.d[room:room + 40] = hdr
        struct.pack_into("<H", pe.d, pe.pe_off + 6, pe.nsec + 1)

    # SizeOfImage 要覆盖到新段末尾
    size_of_image_off = pe.opt_off + 56
    old_soi = struct.unpack_from("<I", pe.d, size_of_image_off)[0]
    new_soi = pe.align_up(new_vaddr + new_vsize, sec_align)
    struct.pack_into("<I", pe.d, size_of_image_off, max(old_soi, new_soi))

    # data directory[2] = 资源目录（NumberOfRvaAndSizes 本来就是 16，不用动）
    n_dirs, dd_base = pe.data_dirs()
    assert n_dirs > 2, "data directory 数量不够，放不下资源项"
    struct.pack_into("<II", pe.d, dd_base + 2 * 8, new_vaddr, new_vsize)

    # 回填资源数据块的 RVA
    buf = bytearray(res_data)
    for i, off in enumerate(offsets):
        struct.pack_into("<I", buf, data_tbl_off + 16 * i, new_vaddr + off)
    res_data = bytes(buf)

    # 写数据。
    #
    # ⚠️ 必须把文件撑到 rawptr + **new_rawsize**（对齐后的长度），不能只写到
    # 数据的实际长度。Windows 加载器会检查"每个段的 SizeOfRawData 范围内的
    # 字节在文件里都存在"，一旦 rawptr+rawsize 越过 EOF，直接拒绝整个文件：
    #     OSError: [WinError 193] %1 不是有效的 Win32 应用程序
    # 这个错误在 CreateProcess 阶段就返回，看着像"PE 头坏了"，
    # 其实只是少补了几千字节的零 —— 二分了半天才定位到。
    need = new_rawptr + new_rawsize
    if len(pe.d) < need:
        pe.d += b"\x00" * (need - len(pe.d))
    pe.d[new_rawptr:new_rawptr + len(res_data)] = res_data
    # 数据末尾到段末尾之间补零（原地写时可能留着上一段的旧字节）
    rest = min(new_rawsize, len(pe.d) - new_rawptr) - len(res_data)
    if rest > 0:
        pe.d[new_rawptr + len(res_data):new_rawptr + len(res_data) + rest] = b"\x00" * rest

    open(exe_path, "wb").write(bytes(pe.d))
    return True


def verify(exe_path):
    pe = PE(open(exe_path, "rb").read())
    sects = pe.sections()
    rsrc = [s for s in sects if s["name"] == ".rsrc"]
    n_dirs, dd_base = pe.data_dirs()
    rva, size = struct.unpack_from("<II", pe.d, dd_base + 2 * 8)
    print("   段数 %d，.rsrc: %s" % (len(sects), "有" if rsrc else "没有"))
    print("   资源目录 RVA %#x 大小 %d 字节" % (rva, size))
    if not rsrc or rva == 0 or size == 0:
        return False
    with open(exe_path, "rb") as f:
        f.seek(rsrc[0]["rawptr"])
        blob = f.read(rsrc[0]["rawsize"])
    # 根目录里应该能找到类型 3/14/16。
    # 注意 IMAGE_RESOURCE_DIRECTORY 的字段偏移：
    #   Characteristics(0) TimeDateStamp(4) MajorVersion(8) MinorVersion(10)
    #   NumberOfNamedEntries(12) NumberOfIdEntries(14)
    # 所以 Id 项数量在 **14**，项表从 16 开始 —— 这里踩过一次，
    # 读 12 会读成 Named 数量（0），看着像"目录空的"。
    n_named, n_id = struct.unpack_from("<HH", blob, 12)
    types = []
    for i in range(n_id):
        tid, _ = struct.unpack_from("<II", blob, 16 + i * 8)
        types.append(tid)
    print("   资源类型:", sorted(types), "(named=%d id=%d)" % (n_named, n_id))
    # RT_ICON 会有多条（每个尺寸一条），所以只看"三类都在"
    uniq = set(types)
    return {RT_GROUP_ICON, RT_VERSION} <= uniq and RT_ICON in uniq


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    exe = sys.argv[1]
    ico = None
    ver = "0.0.0"
    for i, a in enumerate(sys.argv):
        if a == "--ico" and i + 1 < len(sys.argv):
            ico = sys.argv[i + 1]
        if a == "--version" and i + 1 < len(sys.argv):
            ver = sys.argv[i + 1]
    if not ico:
        ico = os.path.join(os.path.dirname(os.path.abspath(exe)), "..",
                           "desktop", "assets", "szudesktop.ico")
    ico = os.path.abspath(ico)
    if not os.path.exists(ico):
        print("!! 找不到图标:", ico)
        sys.exit(1)
    name = os.path.splitext(os.path.basename(exe))[0]
    print(">> 写入图标与版本信息")
    changed = add_resources(exe, ico, ver, name)
    if changed:
        print("   完成，%.1f MB" % (os.path.getsize(exe) / 1024 / 1024))
    print(">> 校验资源段")
    ok = verify(exe)
    print("   %s" % ("通过" if ok else "失败"))
    sys.exit(0 if ok else 1)

"""核对 PNG 内部结构，找"字节一模一样但浏览器不认"的原因。

重点看几项浏览器挑剔的地方：
  - 位深（bit depth）
  - 颜色类型（color type）
  - 有没有交错（interlace）
  - 有没有异常的 chunk（比如 CRC 错、缺失 IEND）

用法: python pnginfo.py <文件> [文件2 ...]
"""
import os
import struct
import sys
import zlib

SIG = b"\x89PNG\r\n\x1a\n"
CT = {0: "灰度", 2: "RGB", 3: "调色板", 4: "灰度+透明", 6: "RGBA"}


def scan(path):
    data = open(path, "rb").read()
    print("== %s  %d 字节" % (os.path.basename(path), len(data)))
    if not data.startswith(SIG):
        print("   !! 不是 PNG 签名")
        return
    pos = 8
    ihdr = None
    bad_crc = []
    chunks = []
    while pos + 8 <= len(data):
        ln, = struct.unpack(">I", data[pos:pos + 4])
        typ = data[pos + 4:pos + 8]
        body = data[pos + 8:pos + 8 + ln]
        crc_stored, = struct.unpack(">I", data[pos + 8 + ln:pos + 12 + ln])
        crc_calc = zlib.crc32(typ + body) & 0xFFFFFFFF
        if crc_stored != crc_calc:
            bad_crc.append(typ.decode("ascii", "replace"))
        chunks.append(typ.decode("ascii", "replace"))
        if typ == b"IHDR":
            w, h, bd, ct, comp, filt, inter = struct.unpack(">IIBBBBB", body)
            ihdr = (w, h, bd, ct, comp, filt, inter)
        pos += 12 + ln
        if typ == b"IEND":
            break

    if ihdr:
        w, h, bd, ct, comp, filt, inter = ihdr
        print("   %dx%d 位深=%d 颜色类型=%d(%s) 压缩=%d 过滤=%d 交错=%d"
              % (w, h, bd, ct, CT.get(ct, "?"), comp, filt, inter))
    print("   chunk: %s" % " ".join(chunks))
    if bad_crc:
        print("   !! CRC 错: %s" % ", ".join(bad_crc))
    else:
        print("   CRC 全部正确")
    if b"IEND" not in [c.encode() for c in chunks]:
        print("   !! 缺 IEND")
    # 尾部有没有多余字节
    tail = len(data) - pos
    if tail > 0:
        print("   !! IEND 之后还有 %d 字节垃圾" % tail)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    for f in sys.argv[1:]:
        scan(f)
        print()

import struct, os

D = r"D:\szuNet\desktop\assets\fonts"
for n in ["svbold.ttf", "svthin.ttf"]:
    d = open(os.path.join(D, n), "rb").read()
    tables = {}
    num = struct.unpack(">H", d[4:6])[0]
    off = 12
    for i in range(num):
        tag = d[off:off + 4].decode("latin1")
        o, ln = struct.unpack(">II", d[off + 8:off + 16])
        tables[tag] = o
        off += 16
    h = tables["head"]
    upem = struct.unpack(">H", d[h + 18:h + 20])[0]
    hh = tables["hhea"]
    asc, desc, gap = struct.unpack(">hhh", d[hh + 4:hh + 10])
    print(n, "unitsPerEm=", upem, "ascender=", asc, "descender=", desc, "lineGap=", gap)
    print("   1px grid unit =", round(upem / 12, 2), "@12px |", round(upem / 16, 2), "@16px |", round(upem / 24, 2), "@24px")
    o = tables["hmtx"]
    print("   advance of 'A'(%d) and 'i'(%d):" % (ord("A"), ord("i")))

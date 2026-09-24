package main

import (
	"testing"

	"github.com/SzuDesktopTeam/szudesktop/internal/portal"
)

// 「没探过」和「探不到」不是一回事，status --json 不能把前者写成后者。
//
// portal.Detect() 在「已经能上外网」时会走快路径提前返回，DormPortalOK /
// TeachPortalOK 只是零值 false，只有 Probed 才表示「门户探测到底跑没跑过」
// （internal/portal/detect.go 的注释把这条写得很清楚）。
//
// 2026-09-23 实测到的不一致：同一台机器、同一时刻，
// `szunet status --json` 报 dorm_portal_ok=false / teaching_portal=false（且没有 probed 键），
// 而同机 `szunet detect --json` 报两个 true —— 两个子命令对同一件事给出相反结论。
// 与 F20 / F22 同族：都是「桌面端修了、CLI 漏了」。
func TestStatusReportOmitsUnprobedPortals(t *testing.T) {
	zone := portal.ZoneOnline
	det := &portal.DetectResult{Zone: zone, InternetOK: true} // Probed 保持零值 false

	out := statusReport(zone, det)

	probed, ok := out["probed"]
	if !ok {
		t.Fatal("报告里必须有 probed 键：消费方要能区分「没探过」和「探不到」")
	}
	if probed != false {
		t.Fatalf("没探测过时 probed 应为 false，得到 %v", probed)
	}
	for _, key := range []string{"dorm_portal_ok", "teaching_portal"} {
		if v, ok := out[key]; ok {
			t.Fatalf("没探测过就不该发布 %s（现在发的是 %v）—— 缺键表示「没测量」，"+
				"发出去的 false 会被读成「探不到」", key, v)
		}
	}
	// 这两个在提前返回前就有值，必须照发，别顺手把整份报告削掉。
	if out["internet_ok"] != true {
		t.Fatalf("外网连通在提前返回之前就测好了，必须照发，得到 %v", out["internet_ok"])
	}
	if out["zone_label"] != zone.Label() {
		t.Fatalf("区域标签应跟着 zone 走，得到 %v", out["zone_label"])
	}
}

// 真的探测过时要照发门户结论，值就是探测结果 ——
// 免得把上一条修成「不管探没探过都永远不发」。
func TestStatusReportPublishesProbedPortals(t *testing.T) {
	zone := portal.ZoneDorm
	det := &portal.DetectResult{
		Zone:          zone,
		InternetOK:    false,
		Probed:        true,
		DormPortalOK:  true,
		TeachPortalOK: false,
	}

	out := statusReport(zone, det)

	if out["probed"] != true {
		t.Fatalf("探测跑过时 probed 应为 true，得到 %v", out["probed"])
	}
	// 关键是值本身：探到了要报 true，探通了但不通也要如实报 false，
	// 不能因为「怕被误读」就不发。
	if v, ok := out["dorm_portal_ok"]; !ok || v != true {
		t.Fatalf("探测到宿舍门户可通时应发 dorm_portal_ok=true，得到 %v（键存在=%v）", v, ok)
	}
	if v, ok := out["teaching_portal"]; !ok || v != false {
		t.Fatalf("探测到教学门户不通时应发 teaching_portal=false，得到 %v（键存在=%v）", v, ok)
	}
}

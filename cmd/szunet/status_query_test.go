package main

import (
	"testing"

	"github.com/SzuDesktopTeam/szudesktop/internal/portal"
)

// 没存账号也必须查在线状态：门户是按出口 IP 回答的，与账号无关，
// 而用户问的正是「我这个出口认证了没」。
//
// 回归的是 2026-09-21 在校园网里实测到的不一致：同一台机器、同一时刻、
// 都没有保存账号，桌面端 /api/status 报「已在线」，而 `szunet status`
// 因为 if credErr == nil 的短路只报「没查到」——F20 修过桌面端，CLI 漏了。
func TestStatusQueriesOnlineEvenWithoutCredentials(t *testing.T) {
	called := 0
	old := queryOnline
	queryOnline = func(zone portal.Zone, srunHost, drcomHost, user, pass string) (*portal.OnlineStatus, error) {
		called++
		if zone != portal.ZoneOnline {
			t.Fatalf("区域应原样传给门户查询，得到 %v", zone)
		}
		if user != "" || pass != "" {
			t.Fatalf("没有账号时应以空账号按出口 IP 查询，得到 user=%q", user)
		}
		return &portal.OnlineStatus{Online: true, IP: "10.20.30.40", DeviceTotal: 1}, nil
	}
	defer func() { queryOnline = old }()

	st, err := statusOnline(portal.ZoneOnline, &options{}, "", "")
	if err != nil {
		t.Fatalf("查询出错: %v", err)
	}
	if called != 1 {
		t.Fatal("没有账号时跳过了在线查询，用户只会看到「没查到」而误判掉线")
	}
	if st == nil || !st.Online || st.IP == "" {
		t.Fatalf("查询结果没有带回来: %+v", st)
	}
}

// 校外或无法判区时不查（不是错误，也不该说成「离线」）。
func TestStatusSkipsQueryOutsideCampus(t *testing.T) {
	called := 0
	old := queryOnline
	queryOnline = func(portal.Zone, string, string, string, string) (*portal.OnlineStatus, error) {
		called++
		return nil, nil
	}
	defer func() { queryOnline = old }()

	st, err := statusOnline(portal.ZoneOutside, &options{}, "", "")
	if err != nil || st != nil {
		t.Fatalf("校外应返回 (nil, nil)，得到 %+v / %v", st, err)
	}
	if called != 1 {
		t.Fatal("区域判断应由门户查询统一处理，调用方不该自己分支")
	}
}

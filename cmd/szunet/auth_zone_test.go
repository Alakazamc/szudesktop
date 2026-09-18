package main

import (
	"github.com/Alakazamc/szudesktop/internal/portal"
	"testing"
)

func TestAuthenticationZoneOnlineAndOverride(t *testing.T) {
	det := &portal.DetectResult{Zone: portal.ZoneOnline, InternetOK: true, Probed: true, DormUsable: true}
	if got := authenticationZone(&options{zone: "auto"}, det); got != portal.ZoneDorm {
		t.Fatalf("online logout would select %s", got)
	}
	if got := authenticationZone(&options{zone: "teaching"}, det); got != portal.ZoneTeaching {
		t.Fatalf("explicit choice lost: %s", got)
	}
	det.DormUsable = false
	if got := authenticationZone(&options{zone: "auto"}, det); got != portal.ZoneUnknown {
		t.Fatalf("unverified authentication selected: %s", got)
	}
}

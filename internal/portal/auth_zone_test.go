package portal

import "testing"

func TestAuthenticationZoneSeparatesConnectivity(t *testing.T) {
	cases := []struct {
		name   string
		result DetectResult
		want   Zone
	}{
		{"Internet alone", DetectResult{Zone: ZoneOnline, InternetOK: true, Probed: true}, ZoneUnknown},
		{"online teaching", DetectResult{Zone: ZoneOnline, InternetOK: true, Probed: true, SrunUsable: true}, ZoneTeaching},
		{"online dorm", DetectResult{Zone: ZoneOnline, InternetOK: true, Probed: true, DormUsable: true}, ZoneDorm},
		{"both follow classification", DetectResult{Zone: ZoneOnline, InternetOK: true, Probed: true, SrunUsable: true, DormUsable: true}, ZoneDorm},
		{"offline portal fallback", DetectResult{Zone: ZoneTeaching, Probed: true}, ZoneTeaching},
		{"outside", DetectResult{Zone: ZoneOutside, Probed: true}, ZoneUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.result.AuthenticationZone(); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

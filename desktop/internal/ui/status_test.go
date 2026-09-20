package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alakazamc/szudesktop/internal/credential"
	"github.com/Alakazamc/szudesktop/internal/portal"
)

type statusTestStore struct {
	guardTestStore
	err error
}

func (s *statusTestStore) Load() (credential.Credentials, error) {
	return s.value, s.err
}

func TestStatusSeparatesSavedCredentialsFromPortalState(t *testing.T) {
	for _, tc := range []struct {
		name, body           string
		zone                 portal.Zone
		saved, known, online bool
		storeErr             error
	}{
		{"online without saved account", `dr1003({"result":1,"user_name":"private-test-account","online_ip":"10.20.30.41"})`, portal.ZoneDorm, false, true, true, credential.ErrNotFound},
		{"offline without saved account", `dr1003({"result":0})`, portal.ZoneDorm, false, true, false, credential.ErrNotFound},
		{"saved account unknown portal", `dr1003({})`, portal.ZoneDorm, true, false, false, nil},
		{"saved account offline portal", `dr1003({"result":0})`, portal.ZoneDorm, true, true, false, nil},
		{"outside campus", "", portal.ZoneOutside, true, false, false, nil},
		{"store unavailable but portal online", `dr1003({"result":1})`, portal.ZoneDorm, false, true, true, errors.New("private-store-detail")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if strings.Contains(r.URL.RawQuery, "private-test-account") || strings.Contains(r.URL.RawQuery, "test-password") {
					t.Error("status query must not send account credentials")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			store := &statusTestStore{err: tc.storeErr}
			if tc.saved {
				store.value = credential.Credentials{Username: "private-test-account", Password: "test-password"}
			}
			s := &Server{
				opts: Options{SrunHost: srv.URL, DrcomHost: srv.URL}, store: store,
				detect: func() *portal.DetectResult { return &portal.DetectResult{Zone: tc.zone, InternetOK: true} },
			}
			rec := httptest.NewRecorder()
			s.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
			var got statusResp
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Saved != tc.saved || got.OnlineKnown != tc.known || got.Online != tc.online || !got.InternetOK {
				t.Fatalf("unexpected status: %+v", got)
			}
			if tc.zone == portal.ZoneOutside {
				if calls != 0 || got.OnlineError != "" {
					t.Fatalf("outside campus should remain unqueried: calls=%d status=%+v", calls, got)
				}
			} else if calls != 1 || (!tc.known && got.OnlineError == "") {
				t.Fatalf("portal query missing or failed silently: calls=%d status=%+v", calls, got)
			}
			if errors.Is(tc.storeErr, credential.ErrNotFound) && got.LastError != "" {
				t.Errorf("not saving credentials is not an error: %s", got.LastError)
			}
			if got.Username != "" || got.OnlineIP != "" {
				t.Error("status revealed account or IP")
			}
			for _, private := range []string{"private-test-account", "test-password", "10.20.30.41", "private-store-detail", srv.URL} {
				if strings.Contains(rec.Body.String(), private) {
					t.Errorf("status revealed private value %q", private)
				}
			}
		})
	}
}

func TestStatusDoesNotCountCommandLineCredentialsAsSaved(t *testing.T) {
	s := &Server{
		opts:   Options{User: "temporary-test-user", Password: "temporary-test-password"},
		store:  &statusTestStore{err: credential.ErrNotFound},
		detect: func() *portal.DetectResult { return &portal.DetectResult{Zone: portal.ZoneOutside} },
	}
	rec := httptest.NewRecorder()
	s.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var got statusResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Saved || got.LastError != "" {
		t.Fatalf("temporary account must not count as saved: %+v", got)
	}
}

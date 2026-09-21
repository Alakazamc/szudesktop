package ui

import (
	"encoding/json"
	"errors"
	"github.com/SzuDesktopTeam/szudesktop/internal/credential"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type ehallTestTransport func(*http.Request) (*http.Response, error)

func (f ehallTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestEhallNeverSendsCookieToUnsafeTarget(t *testing.T) {
	for _, target := range []string{"http://ehall.szu.edu.cn" + undergradScorePath, "https://other.example" + undergradScorePath, "https://ehall.szu.edu.cn/write-action", "https://someone@ehall.szu.edu.cn" + undergradScorePath} {
		t.Run(target, func(t *testing.T) {
			c := newEhallClient("test-only", 0)
			calls := 0
			c.http.Transport = ehallTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 307, Header: http.Header{"Location": {target}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})
			if _, err := c.postForm(undergradScorePath, allRowsForm(1)); err == nil {
				t.Fatal("unsafe redirect accepted")
			}
			if calls != 1 {
				t.Fatalf("cookie sent to redirect target: %d calls", calls)
			}
		})
	}
	c := newEhallClient("test-only", 0)
	c.base = "http://ehall.szu.edu.cn"
	c.http.Transport = ehallTestTransport(func(*http.Request) (*http.Response, error) { t.Fatal("insecure request sent"); return nil, nil })
	if _, err := c.postForm(undergradScorePath, allRowsForm(1)); !errors.Is(err, errUnsafeEhallURL) {
		t.Fatal(err)
	}
}
func TestEhallRequiresExplicitRowsArray(t *testing.T) {
	for _, payload := range []string{`null`, `{}`, `{"rows":null}`, `{"rows":{}}`, `{"rows":[null]}`, `{"rows":[{"KCM":"test"}],"totalSize":0}`} {
		body := `{"code":"0","datas":{"xscjcx":` + payload + `}}`
		if _, err := ehallRows([]byte(body), "xscjcx"); err == nil {
			t.Fatalf("accepted malformed payload %s", payload)
		}
	}
	rows, err := ehallRows([]byte(`{"code":"0","datas":{"xscjcx":{"rows":[]}}}`), "xscjcx")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("explicit empty array rejected")
	}
}
func TestScoreEmptyResponseJSONAndUnknownCompleteness(t *testing.T) {
	for _, total := range []string{"", `,"totalSize":0`} {
		c, _ := newFakeEhall(t, func(string, url.Values) (int, string) {
			return 200, `{"code":"0","datas":{"xscjcx":{"rows":[]` + total + `}}}`
		})
		result, err := readUndergradScore(c)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(result)
		if !strings.Contains(string(data), `"items":[]`) {
			t.Fatalf("empty not an array: %s", data)
		}
		if result.Full != (total != "") {
			t.Fatal("unknown total claimed complete")
		}
	}
}
func TestScoreTruncatedPageCannotClaimComplete(t *testing.T) {
	c, _ := newFakeEhall(t, func(string, url.Values) (int, string) {
		return 200, `{"code":"0","datas":{"xscjcx":{"rows":[{"KCM":"test"}],"totalSize":80}}}`
	})
	result, err := readUndergradScore(c)
	if err != nil || result.Full || result.Total == nil || *result.Total != 80 || result.Note == "" {
		t.Fatalf("bad partial result: %+v %v", result, err)
	}
}
func TestScoresPreserveZeroAndDoNotGuessNumericFields(t *testing.T) {
	c, _ := newFakeEhall(t, func(string, url.Values) (int, string) {
		return 200, `{"code":"0","datas":{"xscjcx":{"rows":[{"KCM":"zero","XF":0,"JD":0},{"KCM":"missing","XFJD":12}],"totalSize":2}}}`
	})
	r, err := readUndergradScore(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range r.Items {
		if x.Name == "zero" && (x.GPA == nil || *x.GPA != 0 || x.Credit == nil || *x.Credit != 0) {
			t.Fatal("lost zero")
		}
		if x.Name == "missing" && (x.GPA != nil || x.Credit != nil) {
			t.Fatal("guessed XFJD meaning")
		}
	}
	data, _ := json.Marshal(r)
	if !strings.Contains(string(data), `"gpa":0`) {
		t.Fatal("zero omitted")
	}
	for _, v := range []any{"NaN", "Inf", "bad", -1.0, 6.0} {
		if _, err := optionalNumber(map[string]any{"JD": v}, "JD", 5); err == nil {
			t.Fatal("invalid point accepted")
		}
	}
}
func TestScoresRejectPartlyUnrecognizableRecords(t *testing.T) {
	c, _ := newFakeEhall(t, func(string, url.Values) (int, string) {
		return 200, `{"code":"0","datas":{"xscjcx":{"rows":[{"KCM":"valid"},{"RENAMED":"missing"}]}}}`
	})
	if _, err := readUndergradScore(c); err == nil {
		t.Fatal("silently discarded an invalid row")
	}
}
func TestSessionCheckUsesSelectedGraduateBusiness(t *testing.T) {
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		if dataset != "xscjcx_dqx" {
			t.Fatal("probed undergraduate application")
		}
		if form.Get("pageSize") != "1" {
			t.Fatal("probe fetched more than needed")
		}
		return 200, `{"code":"0","datas":{"xscjcx_dqx":{"rows":[]}}}`
	})
	s := &Server{session: &memSessionStore{value: credential.Session{Cookie: "test-only"}}, ehallFactory: func(string) *ehallClient { return c }}
	rec := httptest.NewRecorder()
	s.handleSessionCheck(rec, httptest.NewRequest("POST", "/api/session/check?level=graduate", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "研究生") {
		t.Fatal(rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleSessionCheck(rec, httptest.NewRequest("POST", "/api/session/check?level=bad", nil))
	if rec.Code != 400 {
		t.Fatal("invalid level accepted")
	}
}
func TestSchoolPermissionAndExpiryAreDifferent(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   int
	}{
		{403, ``, 403}, {401, ``, 401}, {200, `{"code":"1","msg":"权限不足"}`, 403}, {200, `{"code":"1","msg":"未登录"}`, 401},
	} {
		c, _ := newFakeEhall(t, func(string, url.Values) (int, string) { return tc.status, tc.body })
		s := &Server{session: &memSessionStore{value: credential.Session{Cookie: "test-only"}}, ehallFactory: func(string) *ehallClient { return c }}
		rec := httptest.NewRecorder()
		s.handleScores(rec, httptest.NewRequest("GET", "/api/scores?level=graduate", nil))
		if rec.Code != tc.want {
			t.Fatalf("got %d want %d", rec.Code, tc.want)
		}
	}
}

package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SzuDesktopTeam/szudesktop/internal/credential"
)

// Synthetic records exercise the existing ehall contract; they are not evidence
// that either school's live grade fields or pagination have been accepted.
func scorePageFixture(dataset string, rows []map[string]any, total *int) string {
	payload := map[string]any{"rows": rows}
	if total != nil {
		payload["totalSize"] = *total
	}
	data, _ := json.Marshal(map[string]any{"code": "0", "datas": map[string]any{dataset: payload}})
	return string(data)
}

func TestScoresReadAllReportedPagesForBothLevels(t *testing.T) {
	for _, level := range []string{"undergrad", "graduate"} {
		for _, serverPageSize := range []int{2, scorePageSize} {
			t.Run(fmt.Sprintf("%s/page-size-%d", level, serverPageSize), func(t *testing.T) {
				app, _ := selectScoreApp(level)
				total, calls := 2*serverPageSize+1, 0
				c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
					calls++
					if dataset != app.Dataset || form.Get("pageNumber") != strconv.Itoa(calls) || form.Get("pageSize") != strconv.Itoa(scorePageSize) || form.Get("querySetting") != "[]" {
						t.Errorf("unexpected query: dataset=%s form=%v", dataset, form)
						return 500, ""
					}
					rows := []map[string]any{}
					for i := (calls - 1) * serverPageSize; i < calls*serverPageSize && i < total; i++ {
						row := map[string]any{"JXBID": fmt.Sprintf("test-%03d", i), "XF": 2, "JD": 3.5, "XNXQDM": "2025-2026-1"}
						if level == "graduate" {
							row["KCMC"], row["DYBFZCJ"] = fmt.Sprintf("课程%03d", i), "88"
						} else {
							row["KCM"], row["ZCJ"] = fmt.Sprintf("课程%03d", i), "88"
						}
						rows = append(rows, row)
					}
					return 200, scorePageFixture(dataset, rows, &total)
				})
				result, err := readScore(c, app)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 3 || !result.Full || result.Total == nil || *result.Total != total || result.Fetched != total || len(result.Items) != total || result.Note != "" {
					t.Fatalf("incomplete result: calls=%d result=%+v", calls, result)
				}
				if last := result.Items[total-1]; last.Name != fmt.Sprintf("课程%03d", total-1) || last.Score != "88" || last.Credit == nil || *last.Credit != 2 || last.GPA == nil || *last.GPA != 3.5 {
					t.Fatalf("last-page course not parsed: %+v", last)
				}
			})
		}
	}
}

func TestScoresDoNotGuessPaginationWithoutTotal(t *testing.T) {
	calls := 0
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		calls++
		return 200, scorePageFixture(dataset, []map[string]any{{"KCM": "测试课程"}}, nil)
	})
	result, err := readUndergradScore(c)
	if err != nil || result == nil {
		t.Fatalf("unknown total should retain an explicitly partial result: %+v %v", result, err)
	}
	if calls != 1 || result.Full || result.Total != nil || result.Fetched != 1 || !strings.Contains(result.Note, "不能据此认定成绩已取全") {
		t.Fatalf("unknown completeness lost: calls=%d result=%+v", calls, result)
	}
}

func TestScoresRejectBrokenPaginationWithoutReturningPartialGrades(t *testing.T) {
	for _, tc := range []struct {
		name, nextPayload, want string
	}{
		{"empty-before-total", `{"rows":[],"totalSize":4}`, "条数与总数不一致"},
		{"changed-total", `{"rows":[{"KCM":"C"}],"totalSize":3}`, "总数发生变化或缺失"},
		{"missing-total", `{"rows":[{"KCM":"C"}]}`, "总数发生变化或缺失"},
		{"too-many-rows", `{"rows":[{"KCM":"C"},{"KCM":"D"},{"KCM":"E"}],"totalSize":4}`, "条数与总数不一致"},
		{"repeated-page", `{"rows":[{"KCM":"A"},{"KCM":"B"}],"totalSize":4}`, "重复或重叠"},
		{"overlapping-page", `{"rows":[{"KCM":"B"},{"KCM":"C"}],"totalSize":4}`, "重复或重叠"},
		{"unrecognized-later-record", `{"rows":[{"KCM":"C"},{"RENAMED":"D"}],"totalSize":4}`, "缺少课程名称"},
		{"malformed-later-page", `{"rows":null,"totalSize":4}`, "缺少有效的成绩列表"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
				calls++
				payload := `{"rows":[{"KCM":"A"},{"KCM":"B"}],"totalSize":4}`
				if calls > 1 {
					payload = tc.nextPayload
				}
				return 200, `{"code":"0","datas":{"xscjcx":` + payload + `}}`
			})
			result, err := readUndergradScore(c)
			if err == nil || result != nil || !strings.Contains(err.Error(), tc.want) || calls != 2 {
				t.Fatalf("broken pagination accepted: calls=%d result=%+v err=%v", calls, result, err)
			}
		})
	}
}

func TestScoresPreserveRetakesAcrossPagesAndDeduplicateWithinPage(t *testing.T) {
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		if form.Get("pageNumber") == "1" {
			return 200, `{"code":"0","datas":{"xscjcx":{"rows":[{"JXBID":"same","KCM":"课程","ZCJ":"50"},{"JXBID":"same","KCM":"课程","ZCJ":"50"}],"totalSize":3}}}`
		}
		return 200, `{"code":"0","datas":{"xscjcx":{"rows":[{"JXBID":"same","KCM":"课程","ZCJ":"80"}],"totalSize":3}}}`
	})
	result, err := readUndergradScore(c)
	if err != nil || result == nil {
		t.Fatal(err)
	}
	if !result.Full || result.Fetched != 2 || *result.Total != 3 || result.Items[0].Score != "50" || result.Items[1].Score != "80" {
		t.Fatalf("retake or record count lost: %+v", result)
	}
}

func TestScoresStopAtPageLimitWithoutClaimingComplete(t *testing.T) {
	total, calls := scoreMaxPages+1, 0
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		calls++
		return 200, scorePageFixture(dataset, []map[string]any{{"KCM": fmt.Sprintf("课程%d", calls)}}, &total)
	})
	result, err := readUndergradScore(c)
	if err == nil || result != nil || !strings.Contains(err.Error(), "读取上限") || calls != scoreMaxPages {
		t.Fatalf("unbounded or incomplete pagination: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestScoreAPIRetainsLaterPageFailureStatusAndNoPartialBody(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantStatus int
	}{
		{"expired", 401, "", 401},
		{"forbidden", 403, "", 403},
		{"school-error", 503, "", 502},
		{"login-html", 200, "<html>统一身份认证</html>", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
				calls++
				if calls == 1 {
					return 200, `{"code":"0","datas":{"xscjcx":{"rows":[{"KCM":"部分成绩不可显示为完整"}],"totalSize":2}}}`
				}
				return tc.status, tc.body
			})
			s := &Server{session: &memSessionStore{value: credential.Session{Cookie: "test-only"}}, ehallFactory: func(string) *ehallClient { return c }}
			rec := httptest.NewRecorder()
			s.handleScores(rec, httptest.NewRequest(http.MethodGet, "/api/scores?level=undergrad", nil))
			if calls != 2 || rec.Code != tc.wantStatus || strings.Contains(rec.Body.String(), `"items"`) || strings.Contains(rec.Body.String(), "部分成绩不可显示为完整") {
				t.Fatalf("partial API success on failure: calls=%d status=%d body=%s", calls, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestScoresShareOneDeadlineAcrossPages(t *testing.T) {
	c := newEhallClient("test-only", 0)
	c.http.Timeout = scoreReadTimeout + time.Second
	calls := 0
	var deadline time.Time
	c.http.Transport = ehallTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		got, ok := r.Context().Deadline()
		if !ok || time.Until(got) <= 0 || time.Until(got) > scoreReadTimeout {
			t.Errorf("missing total reading deadline: %v %v", got, ok)
		}
		if calls == 1 {
			deadline = got
		} else if !got.Equal(deadline) {
			t.Error("each page restarted the total timeout")
		}
		total := 2
		body := scorePageFixture("xscjcx", []map[string]any{{"KCM": fmt.Sprintf("课程%d", calls)}}, &total)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	result, err := readUndergradScore(c)
	if err != nil || result == nil || !result.Full || calls != 2 {
		t.Fatalf("paginated read failed: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestScoreAPIRequestCancellationStopsCurrentAndLaterPages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, done := make(chan struct{}), make(chan struct{})
	calls := 0
	c := newEhallClient("test-only", 0)
	c.http.Transport = ehallTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			total := 3
			body := scorePageFixture("xscjcx", []map[string]any{{"KCM": "尚未取全"}}, &total)
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		close(started)
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-time.After(2 * time.Second):
			return nil, errors.New("request cancellation was not propagated")
		}
	})
	s := &Server{session: &memSessionStore{value: credential.Session{Cookie: "test-only"}}, ehallFactory: func(string) *ehallClient { return c }}
	rec := httptest.NewRecorder()
	go func() {
		defer close(done)
		s.handleScores(rec, httptest.NewRequest(http.MethodGet, "/api/scores?level=undergrad", nil).WithContext(ctx))
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("second page never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("read continued after API request cancellation")
	}
	if calls != 2 || rec.Code != 502 || !strings.Contains(rec.Body.String(), "读取已取消") || strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("cancellation returned partial grades or continued: calls=%d status=%d body=%s", calls, rec.Code, rec.Body.String())
	}
}

func TestScoresRespectShorterParentDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	calls := 0
	c := newEhallClient("test-only", 0)
	c.http.Transport = ehallTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	app, _ := selectScoreApp("undergrad")
	result, err := readScoreContext(ctx, c, app)
	if result != nil || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "读取学校成绩超时") || calls > 1 {
		t.Fatalf("parent deadline ignored: calls=%d result=%+v err=%v", calls, result, err)
	}
}

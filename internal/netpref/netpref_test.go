package netpref

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPrefsRoundTrip 确认 ac_id 缓存能按网络标识存下来、读回来。
func TestPrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SZUNET_CONFIG_DIR", dir)

	p := Load()
	if got := p.AcIDFor("172.27.40.1"); got != "" {
		t.Fatalf("干净环境里不该有缓存，却读到 %q", got)
	}

	p.SetAcID("172.27.40.1", "12")
	p.SetAcID("192.168.1.1", "1")
	if err := p.Save(); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	again := Load()
	if got := again.AcIDFor("172.27.40.1"); got != "12" {
		t.Fatalf("路由器这张网应该读到 12，实际 %q", got)
	}
	if got := again.AcIDFor("192.168.1.1"); got != "1" {
		t.Fatalf("另一张网应该读到 1，实际 %q", got)
	}
	if got := again.AcIDFor("10.0.0.1"); got != "" {
		t.Fatalf("没见过的网络不该有值，实际 %q", got)
	}
}

// TestPrefsSurvivesCorruptFile 确认缓存文件坏掉时不影响正常认证。
//
// 这文件只是加速用的。被手动改坏、被同步工具截断，都不该让
// "登录"这件事直接失败 —— 顶多重新发现一次。
func TestPrefsSurvivesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SZUNET_CONFIG_DIR", dir)

	if err := os.WriteFile(filepath.Join(dir, "netpref.json"), []byte("{ 这不是 json"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := Load()
	if p == nil {
		t.Fatal("坏文件应该返回空表，而不是 nil")
	}
	if got := p.AcIDFor("anything"); got != "" {
		t.Fatalf("坏文件不该读出值，实际 %q", got)
	}
}

// TestNetKey 确认网络标识的选取顺序：网关优先，其次出口 IP。
func TestNetKey(t *testing.T) {
	if got := NetKey("172.27.40.1", "172.27.47.91"); got != "172.27.40.1" {
		t.Fatalf("应该优先用网关，实际 %q", got)
	}
	if got := NetKey("", "172.27.47.91"); got != "172.27.47.91" {
		t.Fatalf("没有网关时应退回出口 IP，实际 %q", got)
	}
	if got := NetKey("", ""); got != "" {
		t.Fatalf("都拿不到时应返回空串，实际 %q", got)
	}
}

// TestEgressNotPanic 只是确认在真实机器上取网络标识不会炸。
// 取不到值是允许的（比如受限容器里），但不能影响调用方。
func TestEgressNotPanic(t *testing.T) {
	_ = Egress()
}

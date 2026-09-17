// Package netpref 记住「上次在这张网里认证成功用的是什么参数」。
//
// 起因：深澜认证里的 ac_id（接入点编号）不是固定值 —— 它跟着你插哪个墙口、
// 连哪个无线 AP 走，同一台笔记本在教学区可能是 1，接上路由器换个口就变成 12。
//
// 拿错就会报 "Unknow ac-type"。但我们也不想每次都去盲探一遍，所以把
// 「哪张网 → 哪个 ac_id」记在本地：换网了值对不上，自然走重新发现；
// 没换网就直接复用，省掉一轮请求。
//
// 这里只存 ac_id 这种非敏感信息。账号密码不走这个包，见 internal/credential。
package netpref

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Prefs 是一份按网络标识索引的偏好表。
//
// 键是"网络指纹"（见 NetKey），值是这个网络里上次认证成功的 ac_id。
type Prefs struct {
	AcID map[string]string `json:"ac_id"`
}

func path() (string, error) {
	if d := os.Getenv("SZUNET_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "netpref.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".szunet", "netpref.json"), nil
}

// Load 读取偏好表。文件不存在或格式坏了都返回空表，不报错 ——
// 这只是个加速用的缓存，坏了不该影响正常认证。
func Load() *Prefs {
	p := &Prefs{AcID: map[string]string{}}
	f, err := path()
	if err != nil {
		return p
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return p
	}
	var loaded Prefs
	if err := json.Unmarshal(data, &loaded); err != nil {
		return p
	}
	if loaded.AcID != nil {
		p.AcID = loaded.AcID
	}
	return p
}

// Save 落盘。同样失败不致命，静默返回错误交给调用方决定要不要提示。
func (p *Prefs) Save() error {
	f, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f, data, 0o600)
}

// AcIDFor 取这张网上次成功的 ac_id，没有就返回空串。
func (p *Prefs) AcIDFor(netKey string) string {
	if p == nil || p.AcID == nil {
		return ""
	}
	return p.AcID[netKey]
}

// SetAcID 记下这张网用的 ac_id。
func (p *Prefs) SetAcID(netKey, acID string) {
	if p == nil || netKey == "" || acID == "" {
		return
	}
	if p.AcID == nil {
		p.AcID = map[string]string{}
	}
	p.AcID[netKey] = acID
}

// NetKey 把一份"网络标识"归一成缓存键。
//
// 用网关地址而不是 SSID：有线没有 SSID，手机热点改名更是常事。
// 网关能反映"你插在哪个网段后面"，换墙口/换线路时它会变，
// 正好和 ac_id 的变化同步。
func NetKey(gateways ...string) string {
	for _, g := range gateways {
		g = strings.TrimSpace(g)
		if g != "" {
			return g
		}
	}
	return ""
}

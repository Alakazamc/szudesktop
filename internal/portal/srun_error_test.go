package portal

import (
	"strings"
	"testing"
)

// TestFriendlySrunErrorReadsSucMsg 锁死一个真实踩过的坑。
//
// 背景：深澜服务端并不是把所有信息都放在 error / error_msg 里。
// 有几种情况 error 是 "ok"（看着像成功），真正的结论在 suc_msg；
// 还有的放在 res。老实现只拼 `Error + " " + ErrorMsg` 去匹配，
//
//	→ 匹配不到任何规则，掉进 default 分支，
//	→ 用户看到的是 "认证失败：ok "，什么线索都没有。
//
// 更早的一版还有相反的问题：把 suc_msg 里的 ip_already_online 和
// 账号自己的 already_online 混为一谈，提示成了"该账号本来就在线"，
// 而真实情况是「这个出口 IP 已经有别人的账号挂着了，本次登录压根没生效」。
//
// 这条用例保证四个字段都被纳入匹配，且两种"已在线"给出不同的话。
func TestFriendlySrunErrorReadsSucMsg(t *testing.T) {
	cases := []struct {
		name     string
		resp     srunPortalResp
		mustHave []string
		mustNot  []string
	}{
		{
			name:     "IP 已在线（error=ok，关键信息只在 suc_msg）",
			resp:     srunPortalResp{Error: "ok", SucMsg: "ip_already_online_error"},
			mustHave: []string{"IP", "在线"},
			// 不能提示成"该账号本来就在线"——那是另一回事，
			// 会让用户以为是自己换了号但登录成功了。
			mustNot: []string{"该账号"},
		},
		{
			name:     "账号自己已在线",
			resp:     srunPortalResp{Error: "ok", SucMsg: "already_online"},
			mustHave: []string{"账号", "在线"},
		},
		{
			name:     "challenge 过期（放 error 里）",
			resp:     srunPortalResp{Error: "challenge_expire_error"},
			mustHave: []string{"超时"},
		},
		{
			name:     "auth_info_error",
			resp:     srunPortalResp{Error: "auth_info_error"},
			mustHave: []string{"ac_id"},
		},
		{
			name:     "ac-type 报错",
			resp:     srunPortalResp{Error: "ok", ErrorMsg: "Unknow ac-type: 99"},
			mustHave: []string{"ac_id"},
		},
		{
			name:     "参数不合法",
			resp:     srunPortalResp{Error: "bad_request_parameters"},
			mustHave: []string{"参数"},
		},
		{
			name:     "sign error",
			resp:     srunPortalResp{Error: "sign_error"},
			mustHave: []string{"校验和"},
		},
		{
			name:     "ldap 密码错",
			resp:     srunPortalResp{Error: "ldap auth error"},
			mustHave: []string{"密码"},
		},
		{
			name:     "完全没见过的错误码也要把原文带出来",
			resp:     srunPortalResp{Error: "some_new_error", ErrorMsg: "unexpected"},
			mustHave: []string{"some_new_error"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := friendlySrunError(c.resp)
			for _, want := range c.mustHave {
				if !strings.Contains(got, want) {
					t.Fatalf("friendlySrunError(%+v) = %q，里面应该有 %q", c.resp, got, want)
				}
			}
			for _, bad := range c.mustNot {
				if strings.Contains(got, bad) {
					t.Fatalf("friendlySrunError(%+v) = %q，不该出现 %q", c.resp, got, bad)
				}
			}
		})
	}
}

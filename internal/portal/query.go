package portal

// QueryOnline 按区域查询当前设备的在线状态。
//
// 返回 (nil, nil) 表示"这个区域问不到"（比如人在校外、或校园网整体不通），
// 不是查询失败——调用方据此跳过显示即可。
//
// 为什么要把这段单独抽出来：命令行和桌面界面以前各写了一份一样的判断，
// 而两份都漏了同一种情况——**能上外网时直接跳过查询**。理由写的是
// "不需要认证"，但用户问的其实是"我到底登上了没"，而"能上网"恰恰是
// 已登录的最强证据。偏偏这种时候不查，用户拿到的就是一句干巴巴的
// "没查到"，于是以为自己掉线了，跑去反复点登录，然后被 ip_already_online
// 挡回来——正好掉进整个工具里最让人困惑的那个状态。
//
// 联网时 Detect() 会提前返回、没跑协议指纹，所以不知道当初是哪套协议
// 认证的，那就两套都问一次，谁在线采信谁。两个都是只读查询，代价很小。
func QueryOnline(zone Zone, srunHost, drcomHost, username, password string) (*OnlineStatus, error) {
	switch zone {
	case ZoneTeaching:
		return NewSrunClient(srunHost, username, password).Status()
	case ZoneDorm:
		return NewDrcomClient(drcomHost, username, password).Status()
	case ZoneOnline:
		st, err := NewSrunClient(srunHost, username, password).Status()
		if err == nil && st != nil && st.Online {
			return st, nil
		}
		if dst, derr := NewDrcomClient(drcomHost, username, password).Status(); derr == nil && dst != nil && dst.Online {
			return dst, nil
		}
		return st, err
	default:
		return nil, nil
	}
}

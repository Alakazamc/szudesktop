package portal

import "errors"

// QueryOnline 按区域查询当前设备的在线状态。
//
// 返回 (nil, nil) 表示当前区域未查询（比如人在校外、或无法判区），
// 返回错误表示查询失败。两者都不能当作已确认离线。
//
// 外网可达与校园网认证是独立状态，所以能上外网时仍查询门户。
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
		dst, derr := NewDrcomClient(drcomHost, username, password).Status()
		if derr == nil && dst != nil && dst.Online {
			return dst, nil
		}
		// 只有两套查询都成功且都离线，才能确认出口未认证。
		// 其中一套失败时，另一套的离线结果不能代表失败的那套。
		if err != nil || derr != nil {
			return nil, errors.Join(err, derr)
		}
		return st, nil
	default:
		return nil, nil
	}
}

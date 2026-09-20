// Package portal 实现深大校园网两套认证协议的客户端。
//
// 深大的校园网分成两个互不相同的区域，认证方式完全是两套东西：
//   - 教学区 / 办公区 / 图书馆：深澜（SRun）系统，登录要过三道加密
//   - 宿舍区 / 教工区：Dr.COM 的网页认证（ePortal），一个 GET 请求就够
//
// 这两套协议各自一个文件：srun.go 和 drcom.go。
// 分清楚自己在哪张网，是用这个工具的第一步。
package portal

// Zone 表示设备当前所处的网络区域。
type Zone string

const (
	ZoneTeaching Zone = "teaching" // 教学区：深澜 SRun
	ZoneDorm     Zone = "dorm"     // 宿舍区：Dr.COM 网页认证
	ZoneOnline   Zone = "online"   // 已经能上外网，不需要再认证
	ZoneOutside  Zone = "outside"  // 不在校园网里，或者校园网整体不通
	ZoneUnknown  Zone = "unknown"  // 探测不出来
)

// Label 把区域代号换成给人看的中文说明。
func (z Zone) Label() string {
	switch z {
	case ZoneTeaching:
		return "教学区（深澜 SRun）"
	case ZoneDorm:
		return "宿舍区（Dr.COM 网页认证）"
	case ZoneOnline:
		return "已联网"
	case ZoneOutside:
		return "校外，或校园网不通"
	default:
		return "未知"
	}
}

// Result 是一次认证操作的结果。
type Result struct {
	OK      bool   // 是否成功
	Message string // 给人看的一句话说明
	Raw     string // 服务端原始返回，排错时有用
	AcID    string // 本次实际用的接入点编号（深澜才有意义）

	// AcIDSource 说明上面那个编号是怎么来的：manual / cache / redirect / guess。
	// guess 表示只是从门户页面猜的，未必是你真正所在的接入点。
	AcIDSource string
}

// OnlineStatus 描述账号在某个区域的在线情况。
type OnlineStatus struct {
	Online   bool
	Username string
	IP       string

	// DeviceTotal 是这个出口上登记在线的设备数，Devices 是每台设备的一句话描述。
	//
	// 深澜新版（学生区城市热点）改成按设备登记会话，一个账号可以同时挂多台，
	// 但「一个出口 IP 只能挂一个账号」这条老规矩没变。所以这两个字段是分辨
	// 「这个出口被别人占了」还是「是我自己的旧会话」的关键线索——也是
	// ip_already_online 那类报错唯一能说清楚的具体信息。
	DeviceTotal int
	Devices     []string

	Raw string
}

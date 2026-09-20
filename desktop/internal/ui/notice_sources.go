package ui

import "net/http"

// Verified against the school directory; each source points to its own public column.
// Keep links for colleges whose feeds cannot yet be read, without claiming an empty feed.
type noticeSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Group    string `json:"group"`
	Readable bool   `json:"readable"`
	Note     string `json:"note,omitempty"`
}

var noticeCatalog = []noticeSource{
	{ID: "undergrad", Name: "教务部", URL: "https://jwb.szu.edu.cn/index/jwtz.htm", Group: "university", Readable: true, Note: "本科教学通知"},
	{ID: "graduate", Name: "研究生院", URL: "https://gra.szu.edu.cn/", Group: "university", Readable: true, Note: "研究生院首页公开通知"},
	{ID: "college-fe", Name: "教育学部", URL: "https://fe.szu.edu.cn/xbzx/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-art", Name: "艺术学部", URL: "https://art.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-med", Name: "医学部", URL: "https://med.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-my", Name: "马克思主义学院", URL: "https://my.szu.edu.cn/index/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-eco", Name: "经济学院", URL: "https://eco.szu.edu.cn/xsgz/gztz.htm", Group: "college", Readable: true, Note: "学生工作通知"},
	{ID: "college-law", Name: "法学院", URL: "https://law.szu.edu.cn/xwjc/xygg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-psy", Name: "心理学院", URL: "https://psy.szu.edu.cn/xwgg/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-pes", Name: "体育学院", URL: "https://pes.szu.edu.cn/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-wxy", Name: "人文学院", URL: "https://wxy.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-sfl", Name: "外国语学院", URL: "https://sfl.szu.edu.cn/xssw/jwtz.htm", Group: "college", Readable: false, Note: "该教务栏目未提供可核对的公告日期，请在学院官网查看。"},
	{ID: "college-cmc", Name: "传播学院", URL: "https://cmc.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-math", Name: "数学科学学院", URL: "https://math.szu.edu.cn/bksjy/jwtz.htm", Group: "college", Readable: true, Note: "本科教务通知"},
	{ID: "college-cpoe", Name: "物理与光电工程学院", URL: "https://cpoe.szu.edu.cn/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-chem", Name: "化学与环境工程学院", URL: "https://chem.szu.edu.cn/index/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-bio", Name: "生命与海洋科学学院", URL: "https://bio.szu.edu.cn/xwzx/xytz.htm", Group: "college", Readable: true, Note: "学院通知"},
	{ID: "college-cmce", Name: "机电与控制工程学院", URL: "https://cmce.szu.edu.cn/xydt/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-cmse", Name: "材料学院", URL: "https://cmse.szu.edu.cn/index/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-ceie", Name: "电子与信息工程学院", URL: "https://ceie.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-csse", Name: "计算机与软件学院", URL: "https://csse.szu.edu.cn/", Group: "college", Readable: false, Note: "学院网站暂不接受本应用直接读取，请在学院官网查看。"},
	{ID: "college-ai", Name: "人工智能学院", URL: "https://ai.szu.edu.cn/xwzx/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-saup", Name: "建筑与城市规划学院", URL: "https://saup.szu.edu.cn/dt/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-ce", Name: "土木与交通工程学院", URL: "https://ce.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-ma", Name: "管理学院", URL: "https://ma.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-sg", Name: "政府管理学院", URL: "https://sg.szu.edu.cn/index/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-ias", Name: "高等研究院", URL: "https://ias.szu.edu.cn/xwdt/yjsjy.htm", Group: "college", Readable: true, Note: "研究生教育通知"},
	{ID: "college-lxs", Name: "国际交流学院", URL: "https://lxs.szu.edu.cn/index/tzgg.htm", Group: "college", Readable: true, Note: ""},
	{ID: "college-swift", Name: "深大微众金融科技学院", URL: "https://swift.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
	{ID: "college-safti", Name: "深圳南特金融科技学院", URL: "https://safti.szu.edu.cn/", Group: "college", Readable: false, Note: "暂未接入应用内读取，请在学院官网查看。"},
}

var noticeSources = func() map[string]noticeSource {
	sources := make(map[string]noticeSource, len(noticeCatalog))
	for _, source := range noticeCatalog {
		sources[source.ID] = source
	}
	return sources
}()

func (s *Server) handleCampusNoticeSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"sources": noticeCatalog})
}

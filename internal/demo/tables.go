package demo

// Categorical tables for fabricated values, kept in one file so that changing
// the shape of the demo is a single reviewable diff.

// cnProvince is the province vocabulary. These must be the simplified-Chinese
// names the real GeoLite path produces (internal/geolite/geolite.go:94), because
// the China map matches on them.
var cnProvinces = []string{
	"广东", "北京", "上海", "浙江", "江苏",
	"四川", "湖北", "福建", "山东", "河南",
	"陕西", "湖南", "重庆", "天津", "辽宁",
}

// cnCities per province, index-aligned with cnProvinces.
var cnCities = map[string][]string{
	"广东": {"深圳", "广州", "东莞", "珠海"},
	"北京": {"北京"},
	"上海": {"上海"},
	"浙江": {"杭州", "宁波", "温州"},
	"江苏": {"南京", "苏州", "无锡"},
	"四川": {"成都", "绵阳"},
	"湖北": {"武汉", "宜昌"},
	"福建": {"福州", "厦门", "泉州"},
	"山东": {"青岛", "济南", "烟台"},
	"河南": {"郑州", "洛阳"},
	"陕西": {"西安", "咸阳"},
	"湖南": {"长沙", "株洲"},
	"重庆": {"重庆"},
	"天津": {"天津"},
	"辽宁": {"沈阳", "大连"},
}

// overseasCities keyed by the ISO country code the embedded country database
// returns. Only used to give non-CN traffic a plausible city; the country code
// itself is always the real lookup.
var overseasCities = map[string][]string{
	"US": {"Ashburn", "San Jose", "Dallas", "Seattle"},
	"SG": {"Singapore"},
	"JP": {"Tokyo", "Osaka"},
	"DE": {"Frankfurt", "Berlin"},
	"GB": {"London", "Manchester"},
	"FR": {"Paris", "Marseille"},
	"NL": {"Amsterdam"},
	"AU": {"Sydney", "Melbourne"},
	"KR": {"Seoul"},
	"CA": {"Toronto", "Montreal"},
	"IN": {"Mumbai", "Bengaluru"},
	"BR": {"Sao Paulo"},
}

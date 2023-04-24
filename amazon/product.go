package amazon

type Product struct {
	ASIN                  string              // 产品编号
	Title                 string              // 产品标题
	Price                 string              // 产品价格
	Brand                 string              // 产品品牌
	MerchantInfo          string              // 产品商家信息
	RatingsCount          string              // 产品评价数量
	ListingDate           string              // 上架日期
	Size                  string              // 产品尺寸
	Rating                string              // 产品评分
	ReviewCount           string              // 产品评价数量
	MainRanking           string              // 产品主排名
	SubRanking            string              // 产品子排名
	TechnicalDetails      []map[string]string // 产品技术细节
	AdditionalInformation []map[string]string // 产品附加信息
}

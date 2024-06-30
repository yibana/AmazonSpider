package models

// MerchantItem represents the structure of a merchant item's information
type MerchantItem struct {
	MerchantID    string  `json:"merchant_id" bson:"merchant_id"`
	MarketplaceID string  `json:"marketplaceid" bson:"marketplaceid"`
	Title         string  `json:"title" bson:"title"`
	ASIN          string  `json:"asin" bson:"asin"`
	Price         float64 `json:"price" bson:"price"`
	MonthlySales  int     `json:"monthly_sales" bson:"monthly_sales"`
	ImageURL      string  `json:"image_url" bson:"image_url"`     // 添加商品图片 URL
	ProductURL    string  `json:"product_url" bson:"product_url"` // 添加商品链接
}

package models

// Seller represents the structure of a seller's information
type Seller struct {
	Name         string  `json:"name" bson:"name"`
	Asin         string  `json:"asin" bson:"asin"`
	SellerID     string  `json:"seller_id" bson:"seller_id"`
	Price        float64 `json:"price" bson:"price"`
	UnitPrice    float64 `json:"unit_price,omitempty" bson:"unit_price,omitempty"`
	ShipsFrom    string  `json:"ships_from" bson:"ships_from"`
	Rating       float64 `json:"rating" bson:"rating"`
	RatingCount  int     `json:"rating_count" bson:"rating_count"`
	DeliveryDate string  `json:"delivery_date" bson:"delivery_date"`
}

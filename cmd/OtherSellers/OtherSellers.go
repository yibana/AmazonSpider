package main

import (
	"AmazonSpider/amazon"
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	// Example merchant ID, marketplace, and sort type
	merchantID := "A4VE8BT438UA0"
	marketplace := "A2EUQ1WTGCTBG2"
	sortType := "exact-aware-popularity-rank"
	asin := "B09HH8R1T8"
	scraper, err := amazon.NewAmazonScraper("mongodb://doudian2:doudian20231202@121.41.76.204:27017/", "AmazonScraper", 100)
	if err != nil {
		log.Fatal(err)
	}

	err = scraper.SpiderTask(asin, 100, 2)
	if err != nil {
		log.Fatal(err)
	}

	// Fetch merchant items for the given merchant ID
	items, err := scraper.FetchMerchantItems(merchantID, marketplace, sortType, "1", "")
	if err != nil {
		log.Fatal(err)
	}

	itemsJSON, err := json.MarshalIndent(items, "", "\t")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Merchant Items:", string(itemsJSON))

	sellers, err := scraper.FetchOtherSellers(asin, "")
	if err != nil {
		log.Fatal(err)
	}

	sellersJSON, err := json.MarshalIndent(sellers, "", "\t")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(sellersJSON))
}

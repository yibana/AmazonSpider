package routes

import (
	"AmazonSpider/amazon"
	"AmazonSpider/utils"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

func Readme(c *gin.Context) {
	filePath := filepath.Join(".", "README.md")
	markdownContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error reading file: %s", err.Error()))
		return
	}
	// 渲染模板，并将markdownContent变量替换为动态的Markdown内容
	c.HTML(200, "markdown.html", gin.H{
		"MarkdownContent": string(markdownContent),
		"title":           "README.md",
	})
}

func AllCategorys(c *gin.Context) {
	filepath := filepath.Join(".", "category.json")
	categorys, err := ioutil.ReadFile(filepath)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error reading file: %s", err.Error()))
		return
	}
	c.Data(200, "application/json", categorys)
}

func GetProduct(c *gin.Context) {
	host := c.DefaultQuery("host", "www.amazon.ca")
	asin := c.DefaultQuery("asin", "B08MR2C1T7")

	productURL := fmt.Sprintf("https://%s/dp/%s?th=1&psc=1", host, asin)
	// Create a new collector
	cy := colly.NewCollector(
		colly.AllowedDomains(host),
	)

	if len(utils.ProxyUrl) > 0 {
		// 设置代理IP
		proxyURL, err := url.Parse(utils.ProxyUrl)
		if err != nil {
			log.Fatal(err)
		}
		cy.SetProxyFunc(func(_ *http.Request) (*url.URL, error) {
			return proxyURL, nil
		})
	}

	// Create a product object to store the extracted data
	product := &amazon.Product{}

	// On every product page
	cy.OnHTML("#dp-container", func(e *colly.HTMLElement) {
		ioutil.WriteFile("product.html", []byte(string(e.Response.Body)), 0644)
		product.ASIN = e.ChildAttr("div[data-asin]", "data-asin")
		product.Title = e.ChildText("#productTitle")
		product.Price = e.ChildText("#corePriceDisplay_desktop_feature_div span.a-offscreen")
		product.Brand = strings.TrimPrefix(e.ChildText("a#bylineInfo"), "Brand: ")
		product.MerchantInfo = e.ChildText("#merchant-info")
		product.RatingsCount = e.ChildText("#averageCustomerReviews_feature_div #acrCustomerReviewText")
		product.ListingDate = e.ChildText("#availability .a-color-success")
		product.Rating = e.ChildAttr("#acrPopover", "title")

		e.ForEach("table#productDetails_techSpec_section_1", func(i int, e *colly.HTMLElement) {
			var table = make(map[string]string)
			e.ForEach("tr", func(i int, e *colly.HTMLElement) {
				table[e.ChildText("th")] = e.ChildText("td")
			})
			product.TechnicalDetails = append(product.TechnicalDetails, table)
		})
		e.ForEach("table#productDetails_detailBullets_sections1", func(i int, e *colly.HTMLElement) {
			var table = make(map[string]string)
			e.ForEach("tr", func(i int, e *colly.HTMLElement) {
				table[e.ChildText("th")] = e.ChildText("td")
			})
			product.AdditionalInformation = append(product.AdditionalInformation, table)
		})

		product.MainRanking = e.ChildText("#SalesRank")
		if len(product.TechnicalDetails) > 0 {
			if size, ok := product.TechnicalDetails[0]["Size"]; ok {
				product.Size = size
			}
			if brand, ok := product.TechnicalDetails[0]["Brand"]; ok {
				product.Brand = brand

			}
		}

	})

	// Before making a request
	cy.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	// Start scraping
	err := cy.Visit(productURL)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error Visit: %s", err.Error()))
		return
	}

	// Print the product details
	marshal, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error MarshalIndent: %s", err.Error()))
		return
	}
	c.Data(200, "application/json", marshal)
}

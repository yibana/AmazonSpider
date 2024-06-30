package amazon

import (
	"AmazonSpider/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/PuerkitoBio/goquery"
	fhttp "github.com/saucesteals/fhttp"
	"github.com/saucesteals/mimic"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/proxy"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AmazonScraper represents the Amazon scraper with a reusable HTTP client
type AmazonScraper struct {
	client               *http.Client
	mongoClient          *mongo.Client
	db                   *mongo.Database
	ignoreSellers        sync.Map
	TaskPaused           bool                       // 添加暂停标志
	ItemChan             chan []models.MerchantItem // 添加 channel
	wg                   sync.WaitGroup
	m                    *mimic.ClientSpec
	fetchedSellers       sync.Map
	SpiderLog            *log.Logger      // 添加 SpiderLog
	logBuffer            *strings.Builder // 存储日志内容的 buffer
	SpiderTaskStaring    bool
	spiderTaskCancelFunc context.CancelFunc
	spiderTaskCtx        context.Context
}

// NewAmazonScraper creates a new AmazonScraper with a default HTTP client
func NewAmazonScraper(dbURI, dbName string, maxChanSize int) (*AmazonScraper, error) {
	client := &http.Client{}
	latestVersion := mimic.MustGetLatestVersion(mimic.PlatformWindows)
	m, _ := mimic.Chromium(mimic.BrandChrome, latestVersion)
	mongoClient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(dbURI))
	if err != nil {
		return nil, err
	}
	logBuffer := &strings.Builder{}
	spiderLog := log.New(logBuffer, "SpiderLog: ", log.LstdFlags)
	scraper := &AmazonScraper{
		client:         client,
		mongoClient:    mongoClient,
		db:             mongoClient.Database(dbName),
		ItemChan:       make(chan []models.MerchantItem, maxChanSize), // 初始化 channel
		m:              m,
		fetchedSellers: sync.Map{},
		logBuffer:      logBuffer,
		SpiderLog:      spiderLog,
	}
	err = scraper.loadIgnoreSellers()
	if err != nil {
		return nil, err
	}
	return scraper, nil
}

// convertToNumber converts a string with numbers and units like '1k' or '10.5' to a float64
func convertToNumber(value string) (float64, error) {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)
	value = strings.Replace(value, ",", "", -1)

	re := regexp.MustCompile(`(\d+(\.\d+)?)([kmb]?)`)
	matches := re.FindStringSubmatch(value)

	if len(matches) == 0 {
		return 0, fmt.Errorf("unable to parse number: %s", value)
	}

	number, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	switch matches[3] {
	case "k":
		number *= 1000
	case "m":
		number *= 1000000
	case "b":
		number *= 1000000000
	}

	return number, nil
}

// convertToInt converts a string with numbers and units like '1k' or '10.5' to an int
func convertToInt(value string) (int, error) {
	number, err := convertToNumber(value)
	if err != nil {
		return 0, err
	}
	return int(number), nil
}

func (s *AmazonScraper) GetDB(name string) *mongo.Collection {
	return s.db.Collection(name)
}

func (s *AmazonScraper) loadIgnoreSellers() error {
	collection := s.db.Collection("ignore_sellers")
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(context.TODO())

	s.ignoreSellers = sync.Map{}
	for cursor.Next(context.TODO()) {
		var result struct {
			SellerID string `bson:"seller_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return err
		}
		s.ignoreSellers.Store(result.SellerID, true)
	}
	return nil
}

func (s *AmazonScraper) insertMerchantItem(item models.MerchantItem) error {
	collection := s.db.Collection("merchant_items")
	filter := bson.M{"merchant_id": item.MerchantID, "asin": item.ASIN}
	update := bson.M{"$set": item}
	options := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.TODO(), filter, update, options)
	return err
}

func (s *AmazonScraper) insertSeller(seller models.Seller) error {
	collection := s.db.Collection("sellers")
	filter := bson.M{"asin": seller.Asin, "seller_id": seller.SellerID}
	update := bson.M{"$set": seller}
	options := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.TODO(), filter, update, options)
	return err
}

func (s *AmazonScraper) doRequest(method, _url, proxyAddr string, body io.Reader) ([]byte, error) {
	if true {
		return s.doRequestMini(method, _url, proxyAddr, body)
	} else {
		return s.doRequestRaw(method, _url, proxyAddr, body)
	}
}

func (s *AmazonScraper) doRequestMini(method, _url, proxyAddr string, body io.Reader) ([]byte, error) {

	client := &fhttp.Client{
		Timeout: 10 * time.Second, // 设置超时时间
	}

	transport := &fhttp.Transport{}
	if strings.TrimSpace(proxyAddr) != "" {
		u, err := url.Parse(proxyAddr)
		if err != nil {
			panic(err)
		}
		d := net.Dialer{}
		if strings.HasPrefix(proxyAddr, "http") { // http代理
			transport.DialContext = d.DialContext
			transport.Proxy = fhttp.ProxyURL(u)
		} else {
			dialer, err := proxy.FromURL(u, &d)
			if err != nil {
				return nil, err
			}
			// set our socks5 as the dialer
			transport.Dial = dialer.Dial
		}
		client.Transport = transport
	}
	req, err := fhttp.NewRequest(method, _url, body)
	if err != nil {
		return nil, err
	}
	ua := fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", s.m.Version())

	Header := fhttp.Header{
		"sec-ch-ua":          {s.m.ClientHintUA()},
		"content-type":       {"application/json"},
		"rtt":                {"150"},
		"sec-ch-ua-mobile":   {"?0"},
		"user-agent":         {ua},
		"accept":             {"text/html,*/*"},
		"x-requested-with":   {"XMLHttpRequest"},
		"downlink":           {"10"},
		"ect":                {"4g"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-site":     {"same-origin"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-dest":     {"empty"},
		"accept-encoding":    {"gzip, deflate, br"},
		"accept-language":    {"en,en_US;q=0.9"},
		fhttp.HeaderOrderKey: {
			"sec-ch-ua", "rtt", "sec-ch-ua-mobile",
			"user-agent", "accept", "x-requested-with",
			"downlink", "ect", "sec-ch-ua-platform",
			"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest",
			"accept-encoding", "accept-language",
		},
		fhttp.PHeaderOrderKey: s.m.PseudoHeaderOrder(),
	}
	req.Header = Header
	// Set the necessary headers and cookies
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.AddCookie(&fhttp.Cookie{Name: "i18n-prefs", Value: "CAD"})

	// Send the request using the new HTTP client
	resp, err := client.Do(req)
	if err != nil {
		if proxyAddr != "" {
			// 在使用代理IP时出现异常，更新代理IP
			s.updateProxyError(proxyAddr, err.Error())
			return nil, fmt.Errorf("failed to fetch data using proxy: %s", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if proxyAddr != "" {
			// 在使用代理IP时出现HTTP状态错误，更新代理IP
			s.updateProxyError(proxyAddr, fmt.Sprintf("HTTP status: %s", resp.Status))
		}
		return nil, fmt.Errorf("failed to fetch data: %s", resp.Status)
	}
	encoding := resp.Header["Content-Encoding"]
	content := resp.Header["Content-Type"]

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	Body := cycletls.DecompressBody(bodyBytes, encoding, content)
	return []byte(Body), nil
}

func (s *AmazonScraper) doRequestRaw(method, _url, proxyAddr string, body io.Reader) ([]byte, error) {

	client := &http.Client{
		Timeout: 10 * time.Second, // 设置超时时间
	}

	transport := &http.Transport{}
	if strings.TrimSpace(proxyAddr) != "" {
		u, err := url.Parse(proxyAddr)
		if err != nil {
			panic(err)
		}
		d := net.Dialer{}
		if strings.HasPrefix(proxyAddr, "http") { // http代理
			transport.DialContext = d.DialContext
			transport.Proxy = http.ProxyURL(u)
		} else {
			dialer, err := proxy.FromURL(u, &d)
			if err != nil {
				return nil, err
			}
			// set our socks5 as the dialer
			transport.Dial = dialer.Dial
		}
		client.Transport = transport
	}
	req, err := http.NewRequest(method, _url, body)
	if err != nil {
		return nil, err
	}

	// Set the necessary headers and cookies
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.AddCookie(&http.Cookie{Name: "i18n-prefs", Value: "CAD"})

	// Send the request using the new HTTP client
	resp, err := client.Do(req)
	if err != nil {
		if proxyAddr != "" {
			// 在使用代理IP时出现异常，更新代理IP
			s.updateProxyError(proxyAddr, err.Error())
			return nil, fmt.Errorf("failed to fetch data using proxy: %s", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if proxyAddr != "" {
			// 在使用代理IP时出现HTTP状态错误，更新代理IP
			s.updateProxyError(proxyAddr, fmt.Sprintf("HTTP status: %s", resp.Status))
		}
		return nil, fmt.Errorf("failed to fetch data: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// FetchOtherSellers scrapes the Amazon sellers for a given ASIN
func (s *AmazonScraper) FetchOtherSellers(asin, proxy string) ([]models.Seller, error) {
	url := fmt.Sprintf("https://www.amazon.ca/gp/product/ajax/ref=dp_aod_NEW_mbc?asin=%s&m=&qid=&smid=&sourcecustomerorglistid=&sourcecustomerorglistitemid=&sr=&pc=dp&experienceId=aodAjaxMain", asin)

	body, err := s.doRequest("GET", url, proxy, nil)
	if err != nil {
		return nil, err
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	var sellers []models.Seller

	// Find and iterate over each offer
	doc.Find("#aod-offer").Each(func(i int, s *goquery.Selection) {
		// Extract seller name and trim whitespace
		name := strings.TrimSpace(s.Find("#aod-offer-soldBy .a-link-normal").Text())

		// Extract seller ID from href attribute
		href, exists := s.Find("#aod-offer-soldBy .a-link-normal").Attr("href")
		sellerID := ""
		if exists {
			// Extract the seller ID from the href attribute
			parts := strings.Split(href, "seller=")
			if len(parts) > 1 {
				sellerID = strings.Split(parts[1], "&")[0]
			}
		}

		// Extract price
		priceText := s.Find("#aod-offer-price .a-price .a-offscreen").First().Text()
		price, _ := convertToNumber(priceText)

		// Extract unit price if available
		unitPriceText := s.Find("#aod-offer-price .a-size-mini .a-offscreen").Text()
		unitPrice, _ := convertToNumber(unitPriceText)

		// Extract ships from
		shipsFrom := s.Find("#aod-offer-shipsFrom .a-color-base").Text()

		// Extract rating
		ratingText := s.Find("#aod-offer-seller-rating .a-icon-star-mini").AttrOr("class", "")
		ratingText = strings.TrimPrefix(ratingText, "a-icon a-icon-star-mini a-star-mini-")
		ratingText = strings.Split(ratingText, " ")[0]
		rating, _ := convertToNumber(strings.Replace(ratingText, "-", ".", -1))

		// Extract rating count
		ratingCountText := s.Find("#aod-offer-seller-rating .a-size-small").Text()
		ratingCount, _ := convertToInt(regexp.MustCompile(`\(.*ratings\)`).FindString(ratingCountText))

		// Extract delivery date
		deliveryDate := s.Find(".aod-delivery-promise .a-size-base .a-text-bold").Text()

		sellers = append(sellers, models.Seller{
			Name:         name,
			SellerID:     sellerID,
			Price:        price,
			UnitPrice:    unitPrice,
			ShipsFrom:    shipsFrom,
			Rating:       rating,
			RatingCount:  ratingCount,
			DeliveryDate: deliveryDate,
			Asin:         asin,
		})
	})

	return sellers, nil
}

// FetchMerchantItems scrapes the Amazon merchant items for a given merchant ID, marketplace, and sort type
func (s *AmazonScraper) FetchMerchantItems(merchantID, marketplaceID, sortType, page, proxy string) ([]models.MerchantItem, error) {

	// 通常一页显示16条数据，如果大于等于16则说明可能有下一页
	url := fmt.Sprintf("https://www.amazon.ca/s/query?i=merchant-items&me=%s&s=%s&marketplaceID=%s&page=1&ref=sr_st_%s&page=%s", merchantID, sortType, marketplaceID, sortType, page)

	body, err := s.doRequest("GET", url, proxy, nil)
	if err != nil {
		return nil, err
	}

	// Split the response by '&&&' to separate the different parts
	parts := strings.Split(string(body), "&&&")

	var items []models.MerchantItem

	// Iterate over each part and process it
	for _, part := range parts {
		var result []interface{}
		if err := json.Unmarshal([]byte(part), &result); err != nil {
			continue // Skip this part if it cannot be parsed
		}

		// Process the parsed part to extract item data
		if len(result) > 1 && result[0] == "dispatch" {
			if actionName, ok := result[1].(string); ok && strings.HasPrefix(actionName, "data-main-slot:search-result") {
				if data, ok := result[2].(map[string]interface{}); ok {
					if html, ok := data["html"].(string); ok {
						// Use goquery to parse the HTML part of the response
						doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
						if err != nil {
							return nil, err
						}

						// Find and iterate over each item
						doc.Find(".s-result-item").Each(func(i int, s *goquery.Selection) {
							// Extract item title
							title := s.Find("h2 .a-link-normal .a-text-normal").Text()

							// Extract ASIN from data-asin attribute
							asin, exists := s.Attr("data-asin")
							if !exists {
								asin = ""
							}

							// Extract price
							priceText := s.Find(".a-price .a-offscreen").First().Text()
							price, _ := convertToNumber(priceText)

							// Extract monthly sales
							monthlySalesText := s.Find(".a-size-base.a-color-secondary").Text()
							monthlySales := 0
							if strings.Contains(monthlySalesText, "bought in past month") {
								monthlySalesText = strings.TrimSpace(strings.Split(monthlySalesText, "bought in past month")[0])
								monthlySales, _ = convertToInt(monthlySalesText)
							}
							imageURL, _ := s.Find(".s-image").Attr("src") // 获取商品图片 URL

							// Extract product URL
							productURL, _ := s.Find("h2 .a-link-normal").Attr("href")
							productURL = fmt.Sprintf("https://www.amazon.ca%s", productURL)

							items = append(items, models.MerchantItem{
								Title:         title,
								ASIN:          asin,
								Price:         price,
								MonthlySales:  monthlySales,
								MerchantID:    merchantID,
								MarketplaceID: marketplaceID,
								ImageURL:      imageURL,
								ProductURL:    productURL,
							})
						})
					}
				}
			}
		}
	}

	return items, nil
}

// getNextProxy 获取下一个代理IP，按更新时间排序
func (s *AmazonScraper) getNextProxy() (string, error) {
	collection := s.db.Collection("proxies")
	filter := bson.M{"status": "enabled", "error_count": bson.M{"$lt": 10}}
	opts := options.FindOne().SetSort(bson.D{{"updated_at", 1}})

	var proxy models.Proxy
	err := collection.FindOne(context.TODO(), filter, opts).Decode(&proxy)
	if err != nil {
		return "", fmt.Errorf("no available proxies: %v", err)
	}

	// 更新代理的更新时间
	update := bson.M{"$set": bson.M{"updated_at": time.Now()}}
	_, err = collection.UpdateOne(context.TODO(), bson.M{"ip": proxy.IP}, update)
	if err != nil {
		return "", fmt.Errorf("failed to update proxy: %v", err)
	}

	return proxy.IP, nil
}

// 采集任务的线程子任务之一
func (s *AmazonScraper) task_add_seller(asin, proxy string, taskChan *chan models.Seller, waitGroup *sync.WaitGroup, asinChan *chan string) error {
	defer waitGroup.Done()
	select {
	case <-*asinChan:
	case <-s.spiderTaskCtx.Done():
		return errors.New("task cancelled")
	default:
		break
	}
	s.SpiderLog.Printf("采集跟卖信息: Asin:%s, Proxy:%s", asin, proxy)
	sellers, err := s.FetchOtherSellers(asin, proxy)
	if err != nil {
		return err
	}
	for _, seller := range sellers {
		if _, ok := s.ignoreSellers.Load(seller.SellerID); ok { // 用户设置的忽略列表
			continue
		}
		if _, ok := s.fetchedSellers.Load(seller.SellerID); ok { // 本次已经采集过的列表
			continue
		}
		s.fetchedSellers.Store(seller.SellerID, true)
		select {
		case *taskChan <- seller:
		case <-s.spiderTaskCtx.Done():
			return errors.New("task cancelled")
		}

		err = s.insertSeller(seller) // 记录到数据库，忽略错误
		if err != nil {
			log.Println(err.Error())
		}
	}
	return nil
}

func (s *AmazonScraper) task_fetch_seller_merchantItems(taskChan *chan models.Seller, salesThreshold int, waitGroup *sync.WaitGroup, asinChan *chan string) error {
	defer waitGroup.Done()

	task_back := func(seller models.Seller) {
		go func() {
			select {
			case *taskChan <- seller: // 防止被阻塞:
				break
			case <-s.spiderTaskCtx.Done():
				return
			}

		}()
	}

	//为每次FetchMerchantItems请求加一个间隔限制，防止被503
	rateLimiter := time.NewTicker(time.Millisecond * 500)

	for {
		select {
		case seller := <-*taskChan:
			proxy, err := s.getNextProxy()
			if err != nil {
				task_back(seller)
				return fmt.Errorf("getNextProer %v", err)
			}

			for page := 0; page < 10; page++ {
				for s.TaskPaused { // 如果任务暂停
					<-rateLimiter.C // 等待下一次请求时间
				}
				select {
				case <-rateLimiter.C:
					break
				case <-s.spiderTaskCtx.Done():
					return errors.New("reset task")
				}
				s.SpiderLog.Printf("采集卖家商品: SellerID:%s Page:%d Proxy:%s", seller.SellerID, page, proxy)

				items, err := s.FetchMerchantItems(seller.SellerID,
					"A2EUQ1WTGCTBG2", "relevanceblender",
					fmt.Sprintf("%d", page), proxy)
				if err != nil {
					task_back(seller)
					return fmt.Errorf("FetchMerchantItems: %v", err)
				}

				if len(items) == 0 {
					break
				}

				select {
				case s.ItemChan <- items:
				default:
					s.SpiderLog.Println("item channel is full, discarding items")
				}

				for _, item := range items {
					err := s.insertMerchantItem(item)
					if err != nil {
						return fmt.Errorf("inserting merchant item: %v", err)
					}

					if item.MonthlySales > salesThreshold {
						go func() {

							select {
							case *asinChan <- item.ASIN: // 如果满了会被阻塞
								break
							case <-s.spiderTaskCtx.Done():
								return
							}
							waitGroup.Add(1)
							err2 := s.task_add_seller(item.ASIN, proxy, taskChan, waitGroup, asinChan)
							if err2 != nil {
								s.SpiderLog.Printf("task_add_seller failed: %v", err2)
							}
						}()
					}
				}
			}
		case <-s.spiderTaskCtx.Done():
			return errors.New("task cancelled")
		}
	}

	return nil
}

func (s *AmazonScraper) GetSpiderLogs() string {
	return s.logBuffer.String()
}

func (s *AmazonScraper) ResetSpiderTask() error {
	if s.SpiderTaskStaring {
		s.spiderTaskCancelFunc()
	}

	return nil
}

func (s *AmazonScraper) SpiderTask(asin string, salesThreshold int, threads int) error {
	if s.SpiderTaskStaring {
		return errors.New("spider task is already running")
	}
	s.SpiderTaskStaring = true
	defer func() {
		s.SpiderTaskStaring = false
		s.TaskPaused = false
	}()

	s.fetchedSellers = sync.Map{}
	s.logBuffer.Reset()
	s.loadIgnoreSellers()
	ctx, cancelFunc := context.WithCancel(context.Background())
	s.spiderTaskCancelFunc = cancelFunc
	s.spiderTaskCtx = ctx

	s.SpiderLog.Println("Starting SpiderTask...")
	// 初始化任务
	proxy, err := s.getNextProxy()
	if err != nil {
		s.SpiderLog.Printf("Error getting proxy: %v", err)
		return err
	}

	taskChan := make(chan models.Seller, threads) // 任务通道
	asinChan := make(chan string, threads)
	var waitGroup sync.WaitGroup

	// 启动工作协程
	for i := 0; i < threads; i++ {
		waitGroup.Add(1)
		go func() {
			err2 := s.task_fetch_seller_merchantItems(&taskChan, salesThreshold, &waitGroup, &asinChan)
			if err2 != nil {
				s.SpiderLog.Printf("Error fetching merchant items: %v", err2)
			}
		}()
	}

	// 初始化第一个任务
	waitGroup.Add(1)
	err = s.task_add_seller(asin, proxy, &taskChan, &waitGroup, &asinChan)
	if err != nil {
		s.SpiderLog.Printf("Error adding seller: %v", err)
	}

	waitGroup.Wait()
	//close(taskChan) // 关闭任务通道以结束工作协程
	return nil
}

func (s *AmazonScraper) updateProxyError(addr string, message string) {
	collection := s.db.Collection("proxies")
	filter := bson.M{"ip": addr}
	update := bson.M{
		"$set": bson.M{
			"message": message,
		},
		"$inc": bson.M{
			"error_count": 1, // 自增字段，每次调用时+1
		},
	}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Failed to update proxy status: %v\n", err)
	}
}

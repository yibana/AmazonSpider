package main

import (
	"AmazonSpider/amazon"
	"AmazonSpider/models"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"strings"
	"time"
)

type WSRequest struct {
	Type           string `json:"type"`
	ASIN           string `json:"asin,omitempty"`
	SalesThreshold int    `json:"salesThreshold,omitempty"`
	Threads        int    `json:"threads,omitempty"`
}

type WSResponse struct {
	Type   string                `json:"type"`
	Data   []models.MerchantItem `json:"data,omitempty"`
	Error  string                `json:"error,omitempty"`
	Status string                `json:"status,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TaskManager struct {
	scraper  *amazon.AmazonScraper
	scraping bool
	stopCh   chan struct{}
}

func NewTaskManager(scraper *amazon.AmazonScraper) *TaskManager {
	return &TaskManager{scraper: scraper, stopCh: make(chan struct{})}
}

func (tm *TaskManager) HandleWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to set websocket upgrade: ", err)
		return
	}
	defer conn.Close()

	for {
		var req WSRequest
		err := conn.ReadJSON(&req)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				log.Println("WebSocket closed unexpectedly:", err)
			}
			break
		}

		switch req.Type {
		case "start":
			if tm.scraping {
				conn.WriteJSON(WSResponse{Type: "error", Error: "A scraping task is already running"})
				continue
			}

			tm.scraping = true

			go func() {
				defer func() {
					tm.scraping = false
				}()

				err := tm.scraper.SpiderTask(req.ASIN, req.SalesThreshold, req.Threads)
				if err != nil {
					log.Println("Error fetching and saving ASIN data: ", err)
					conn.WriteJSON(WSResponse{Type: "error", Error: err.Error()})
					return
				} else {
					conn.WriteJSON(WSResponse{Type: "error", Error: "采集已经结束"})
				}
			}()

			conn.WriteJSON(WSResponse{Type: "status", Status: "started"})

		case "pause":
			tm.scraper.TaskPaused = true
			conn.WriteJSON(WSResponse{Type: "status", Status: "paused"})

		case "resume":
			tm.scraper.TaskPaused = false
			conn.WriteJSON(WSResponse{Type: "status", Status: "resumed"})

		case "status":
			if tm.scraping {
				if tm.scraper.TaskPaused {
					conn.WriteJSON(WSResponse{Type: "status", Status: "paused"})
				} else {
					conn.WriteJSON(WSResponse{Type: "status", Status: "started"})
				}
			} else {
				conn.WriteJSON(WSResponse{Type: "status", Status: "completed"})
			}
		}

		go func() {
			for tm.scraping {
				select {
				case items := <-tm.scraper.ItemChan:
					conn.WriteJSON(WSResponse{Type: "data", Data: items})
				case <-time.After(5 * time.Second):
					// Check if scraping is still active
					if !tm.scraping {
						return
					}
				}
			}
		}()
	}
}

type QueryParams struct {
	PriceMin        float64 `form:"price_min"`
	PriceMax        float64 `form:"price_max"`
	MonthlySalesMin int     `form:"monthly_sales_min"`
	MonthlySalesMax int     `form:"monthly_sales_max"`
	ASIN            string  `form:"asin"`
	MerchantID      string  `form:"merchant_id"`
	Page            int     `form:"page"`
	Limit           int     `form:"limit"`
}
type IgnoreSeller struct {
	SellerID string `json:"seller_id" bson:"seller_id"`
}

func (tm *TaskManager) Query(c *gin.Context) {
	var params QueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := tm.scraper.GetDB("merchant_items")
	filter := bson.M{}

	if params.PriceMin > 0 {
		filter["price"] = bson.M{"$gte": params.PriceMin}
	}
	if params.PriceMax > 0 {
		if filter["price"] != nil {
			filter["price"].(bson.M)["$lte"] = params.PriceMax
		} else {
			filter["price"] = bson.M{"$lte": params.PriceMax}
		}
	}
	if params.MonthlySalesMin > 0 {
		filter["monthly_sales"] = bson.M{"$gte": params.MonthlySalesMin}
	}
	if params.MonthlySalesMax > 0 {
		if filter["monthly_sales"] != nil {
			filter["monthly_sales"].(bson.M)["$lte"] = params.MonthlySalesMax
		} else {
			filter["monthly_sales"] = bson.M{"$lte": params.MonthlySalesMax}
		}
	}
	if params.ASIN != "" {
		filter["asin"] = params.ASIN
	}
	if params.MerchantID != "" {
		filter["merchant_id"] = params.MerchantID
	}

	total, err := collection.CountDocuments(context.TODO(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	findOptions := options.Find()
	findOptions.SetSkip(int64(params.Page * params.Limit))
	findOptions.SetLimit(int64(params.Limit))

	cursor, err := collection.Find(context.TODO(), filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(context.TODO())

	var results []bson.M
	if err := cursor.All(context.TODO(), &results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"data":  results,
	})
}

var client *mongo.Client
var db *mongo.Database

func main() {

	r := gin.Default()

	// 创建 AmazonScraper 实例
	dbURI := "mongodb://doudian2:doudian20231202@121.41.76.204:27017/"
	dbName := "AmazonScraper"

	var err error
	client, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(dbURI))
	if err != nil {
		log.Fatal(err)
	}
	db = client.Database(dbName)
	//ignoreSellersList := []string{"A2UNSMLO5W5JHM"}
	maxChanSize := 100
	scraper, err := amazon.NewAmazonScraper(dbURI, dbName, maxChanSize)
	if err != nil {
		log.Fatal(err)
	}

	taskManager := NewTaskManager(scraper)

	r.GET("/ws", taskManager.HandleWS)

	r.GET("/query", taskManager.Query)

	// Ignore Sellers List APIs
	r.GET("/ignore-sellers", getIgnoreSellers)
	r.POST("/ignore-sellers", addIgnoreSeller)
	r.DELETE("/ignore-sellers/:seller_id", deleteIgnoreSeller)

	// Proxies APIs
	r.GET("/proxies", getProxies)
	r.POST("/proxies", addProxies)
	r.PUT("/proxies", updateProxyStatus)
	r.DELETE("/proxies", deleteProxy)

	r.Run(":8080")
}

func getIgnoreSellers(c *gin.Context) {
	var ignoreSellers = []IgnoreSeller{}
	collection := db.Collection("ignore_sellers")
	cursor, err := collection.Find(context.TODO(), bson.D{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(context.TODO())
	for cursor.Next(context.TODO()) {
		var seller IgnoreSeller
		cursor.Decode(&seller)
		ignoreSellers = append(ignoreSellers, seller)
	}
	c.JSON(http.StatusOK, ignoreSellers)
}

func addIgnoreSeller(c *gin.Context) {
	var seller IgnoreSeller
	if err := c.ShouldBindJSON(&seller); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	collection := db.Collection("ignore_sellers")
	filter := bson.M{"seller_id": seller.SellerID}
	update := bson.M{"$set": seller}
	options := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.TODO(), filter, update, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, seller)
}

func deleteIgnoreSeller(c *gin.Context) {
	sellerID := c.Param("seller_id")
	collection := db.Collection("ignore_sellers")
	_, err := collection.DeleteOne(context.TODO(), bson.M{"sellerid": sellerID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": sellerID})
}

func getProxies(c *gin.Context) {
	var proxies = make([]models.Proxy, 0)
	collection := db.Collection("proxies")
	cursor, err := collection.Find(context.TODO(), bson.D{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(context.TODO())
	for cursor.Next(context.TODO()) {
		var proxy models.Proxy
		cursor.Decode(&proxy)
		proxies = append(proxies, proxy)
	}
	c.JSON(http.StatusOK, proxies)
}

func addProxies(c *gin.Context) {
	var proxyList []string
	if err := c.ShouldBindJSON(&proxyList); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建批量操作
	var _models []mongo.WriteModel
	for _, p := range proxyList {
		proxy := p
		if !strings.HasPrefix(proxy, "http") && !strings.HasPrefix(proxy, "socks") {
			proxy = fmt.Sprintf("socks5://%s", proxy)
		}
		filter := bson.M{"ip": p}
		update := bson.M{"$set": models.Proxy{IP: proxy, Status: "enabled", UpdatedAt: time.Now()}}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
		_models = append(_models, model)
	}

	collection := db.Collection("proxies")
	opts := options.BulkWrite().SetOrdered(false)
	_, err := collection.BulkWrite(context.TODO(), _models, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Proxies added/updated successfully"})
}

func updateProxyStatus(c *gin.Context) {
	var proxy models.Proxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	collection := db.Collection("proxies")
	_, err := collection.UpdateOne(context.TODO(), bson.M{"ip": proxy.IP}, bson.M{"$set": bson.M{"status": proxy.Status, "message": proxy.Message}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": proxy.IP})
}

func deleteProxy(c *gin.Context) {
	var proxy models.Proxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	collection := db.Collection("proxies")
	_, err := collection.DeleteOne(context.TODO(), bson.M{"ip": proxy.IP})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": proxy.IP})
}

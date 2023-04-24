package main

import (
	"AmazonSpider/routes"
	"github.com/gin-gonic/gin"
)

// 使用gin框架
func main() {
	r := gin.Default()

	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello world!",
		})
	})
	r.GET("/readme", routes.Readme)
	r.GET("/categorys", routes.AllCategorys)
	r.GET("/product", routes.GetProduct)

	r.Run()
}

//func createCollection() (*mongo.Collection, error) {
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//
//	client, err := mongo.Connect(ctx, options.Client().ApplyURI(utils.MongodbUrl))
//	if err != nil {
//		return nil, err
//	}
//
//	collection := client.Database("mydb").Collection("categories")
//
//	return collection, nil
//}
//
//func Save(category *amazon.Category) error {
//	collection, err := createCollection()
//	if err != nil {
//		return err
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//
//	if _, err := collection.InsertOne(ctx, category); err != nil {
//		return err
//	}
//
//	return nil
//}

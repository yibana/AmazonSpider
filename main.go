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

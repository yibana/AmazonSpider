package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// 启动无头浏览器
	browser := rod.New().MustConnect()
	defer browser.MustClose()

	// 创建一个新页面并设置随机请求头
	page := stealth.MustPage(browser).MustNavigate("https://www.amazon.ca/")

	// 获取页面标题
	title := page.MustInfo().Title
	fmt.Println("Page Title:", title)
}

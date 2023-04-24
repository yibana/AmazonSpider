package spider

import (
	"AmazonSpider/amazon"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)
import "github.com/gocolly/colly"

var root = &amazon.Category{
	Name: "Best Sellers",
	Url:  "https://www.amazon.ca/Best-Sellers-generic/zgbs/?ref=nav_cs_bestsellers",
	Sub:  make([]*amazon.Category, 0),
}

func main() {
	rootCategory := &amazon.CategoryManger{Uniqueness: make(map[string]bool)}
	rootCategory.Add(nil, root)

	go func() {
		for {
			rootCategory.SaveRootToFile("category.json")
			time.Sleep(time.Second * 10)
		}
	}()

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})
	// 设置代理IP
	proxyURL, err := url.Parse("http://127.0.0.1:10809")
	if err != nil {
		log.Fatal(err)
	}
	c.SetProxyFunc(func(_ *http.Request) (*url.URL, error) {
		return proxyURL, nil
	})

	c.OnHTML("div[role=tree]", func(e *colly.HTMLElement) {
		body := string(e.Response.Body)
		if strings.Contains(body, "Sorry, we just need to make sure you're not a robot. For best results, please make sure your browser is accepting cookies.") {
			ioutil.WriteFile("error.html", []byte(body), 0644)
			log.Fatal("Sorry, we just need to make sure you're not a robot. For best results, please make sure your browser is accepting cookies")
		}
		// 获取所有的子节点
		parent := e.Request.Ctx.GetAny("parent")

		var treeitemCount, treeitemCount_a int
		var nodes []*amazon.Category
		e.ForEach("div[role=group] div[role=treeitem]", func(i int, element *colly.HTMLElement) {
			treeitemCount++

		})

		e.ForEach("div[role=group] div[role=treeitem] a", func(i int, element *colly.HTMLElement) {
			node := &amazon.Category{
				Url:  element.Request.AbsoluteURL(element.Attr("href")),
				Name: element.Text,
				Sub:  make([]*amazon.Category, 0),
			}
			nodes = append(nodes, node)
			treeitemCount_a++
		})

		if treeitemCount == treeitemCount_a {
			for _, node := range nodes {
				rootCategory.Add(parent, node)
			}
			for _, node := range nodes {
				if parent != nil && parent.(*amazon.Category).Maybe_end { // 如果父节点已经结束，那么子节点也不需要再访问了
					continue
				}
				request, _ := e.Request.New("GET", node.Url, nil)
				request.Ctx.Put("parent", node)
				log.Printf("visit %s name:%s", node.Url, node.Name)
				request.Visit(node.Url)
			}
		} else {
			if parent != nil && parent.(*amazon.Category).Parent != nil {
				parent.(*amazon.Category).Parent.Maybe_end = true
			}
		}

	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})
	c.Visit(root.Url)
	rootCategory.SaveRootToFile("category.json")
}

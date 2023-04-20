package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)
import "github.com/gocolly/colly"

type Category struct {
	Url       string
	Name      string
	Sub       []*Category
	maybe_end bool // 有可能是最后一级
	parent    *Category
}

type CategoryManger struct {
	root       *Category
	lock       sync.Mutex
	Uniqueness map[string]bool
	cansave    bool
}

func (c *CategoryManger) SaveRootToFile(fn string) error {
	if !c.cansave {
		return nil
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.root != nil {
		marshal, err := json.Marshal(c.root)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(fn, marshal, 0644)
	}
	return nil
}

func (c *CategoryManger) UniqueUrl(_url string) string {
	// 提取url截至到ref
	parse, err := url.Parse(_url)
	if err != nil {
		return ""
	}
	// 返回去掉query的url
	_url = fmt.Sprintf("%s://%s%s", parse.Scheme, parse.Host, parse.Path)

	// 取url中截至到ref的部分
	if strings.Contains(_url, "ref=") {
		_url = _url[:strings.Index(_url, "ref=")]
	}

	return _url
}

func (c *CategoryManger) Add(parent interface{}, category *Category) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cansave = true
	var p *Category
	if parent == nil {
		p = c.root
	} else {
		p = parent.(*Category)
	}
	category.parent = p
	p.Sub = append(p.Sub, category)
}

func (c *CategoryManger) AddCategory(parentUrl string, category *Category) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	uniquene := category.Url
	if c.Uniqueness[uniquene] {
		return false
	}
	if c.root != nil && c.root.Url == category.Url {
		return false
	}
	c.cansave = true
	c.Uniqueness[uniquene] = true
	if c.root == nil {
		c.root = category
	} else {
		parent := c.findParent(c.root, parentUrl)
		if parent != nil {
			parent.Sub = append(parent.Sub, category)
		} else {
			return false
		}
	}
	return true
}

func (c *CategoryManger) findParent(category *Category, url string) *Category {
	if strings.HasPrefix(url, category.Url) {
		return category
	}
	if len(category.Sub) > 0 {
		for _, sub := range category.Sub {
			parent := c.findParent(sub, url)
			if parent != nil {
				return parent
			}
		}
	}
	return nil
}

var root = &Category{
	Name: "Best Sellers",
	Url:  "https://www.amazon.ca/Best-Sellers-generic/zgbs/?ref=nav_cs_bestsellers",
	Sub:  make([]*Category, 0),
}

func main() {
	rootCategory := &CategoryManger{Uniqueness: make(map[string]bool)}
	rootCategory.AddCategory("", root)

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
	// Find and visit all links
	//c.OnHTML("a[href]", func(e *colly.HTMLElement) {
	//	body := string(e.Response.Body)
	//	if strings.Contains(body, "Sorry, we just need to make sure you're not a robot. For best results, please make sure your browser is accepting cookies.") {
	//		ioutil.WriteFile("error.html", []byte(body), 0644)
	//		return
	//	}
	//
	//	href := e.Attr("href")
	//	log.Println(href)
	//	visitUrl := e.Request.AbsoluteURL(href)
	//	if strings.HasPrefix(href, "/Best-Sellers-") && len(e.Text) > 1 {
	//
	//		if strings.Contains(e.Text, "Next page") {
	//			return
	//		}
	//		if strings.Contains(e.Text, "Previous Page") {
	//			return
	//		}
	//
	//		log.Println(e.Text, href, e.Request.URL)
	//		node := &Category{
	//			Url:  rootCategory.UniqueUrl(visitUrl),
	//			Name: e.Text,
	//			Sub:  make([]*Category, 0),
	//		}
	//		addSucc := rootCategory.AddCategory(e.Request.URL.String(), node)
	//		if !rootCategory.IsRootUrl(visitUrl) && addSucc {
	//			e.Request.Visit(href)
	//		}
	//	}
	//})

	c.OnHTML("div[role=tree]", func(e *colly.HTMLElement) {
		body := string(e.Response.Body)
		if strings.Contains(body, "Sorry, we just need to make sure you're not a robot. For best results, please make sure your browser is accepting cookies.") {
			ioutil.WriteFile("error.html", []byte(body), 0644)
			log.Fatal("Sorry, we just need to make sure you're not a robot. For best results, please make sure your browser is accepting cookies")
		}
		// 获取所有的子节点
		parent := e.Request.Ctx.GetAny("parent")

		var treeitemCount, treeitemCount_a int
		var nodes []*Category
		e.ForEach("div[role=group] div[role=treeitem]", func(i int, element *colly.HTMLElement) {
			treeitemCount++

		})

		e.ForEach("div[role=group] div[role=treeitem] a", func(i int, element *colly.HTMLElement) {
			node := &Category{
				Url:  element.Request.AbsoluteURL(element.Attr("href")),
				Name: element.Text,
				Sub:  make([]*Category, 0),
			}
			nodes = append(nodes, node)
			treeitemCount_a++
		})

		if treeitemCount == treeitemCount_a {
			for _, node := range nodes {
				rootCategory.Add(parent, node)
			}
			for _, node := range nodes {
				if parent != nil && parent.(*Category).maybe_end { // 如果父节点已经结束，那么子节点也不需要再访问了
					continue
				}
				request, _ := e.Request.New("GET", node.Url, nil)
				request.Ctx.Put("parent", node)
				log.Printf("visit %s name:%s", node.Url, node.Name)
				request.Visit(node.Url)
			}
		} else {
			if parent != nil && parent.(*Category).parent != nil {
				parent.(*Category).parent.maybe_end = true
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

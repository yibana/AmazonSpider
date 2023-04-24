package amazon

import (
	"encoding/json"
	"io/ioutil"
	"sync"
)

type Category struct {
	Url       string
	Name      string
	Sub       []*Category
	Maybe_end bool      `json:"-"` // 有可能是最后一级
	Parent    *Category `json:"-"`
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

func (c *CategoryManger) Add(parent interface{}, category *Category) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.root == nil {
		c.root = category
		return
	}

	c.cansave = true
	var p *Category
	if parent == nil {
		p = c.root
	} else {
		p = parent.(*Category)
	}
	category.Parent = p
	p.Sub = append(p.Sub, category)
}

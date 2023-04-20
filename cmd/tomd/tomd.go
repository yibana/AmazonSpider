package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Category struct {
	Url  string
	Name string
	Sub  []*Category
}

func writeCategory(w *os.File, category *Category, level int) {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "  "
	}
	fmt.Fprintf(w, "%s- [%s](%s)\n", indent, category.Name, category.Url)
	for _, sub := range category.Sub {
		writeCategory(w, sub, level+1)
	}
}

func writeCategoriesToFile(categories []*Category, filename string) error {
	w, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer w.Close()
	fmt.Fprintf(w, "# Amazon Categories\n\n")
	for _, category := range categories {
		writeCategory(w, category, 0)
	}
	return nil
}

func main() {
	root := Category{}
	bytes, err2 := ioutil.ReadFile("category.json")
	if err2 != nil {
		fmt.Println(err2)
		return
	}
	err := json.Unmarshal(bytes, &root)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = writeCategoriesToFile(root.Sub, "README.md")
	if err != nil {
		fmt.Println(err)
		return
	}

	data, err := ioutil.ReadFile("README.md")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data))
}

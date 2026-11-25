// Najmniejszy program walidujący dokument według schematu: cB-Sigma/xsd (przykład z README).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cB-Sigma/xsd"
)

func main() {
	s, err := xsd.Load(os.DirFS(filepath.Dir(os.Args[1])), filepath.Base(os.Args[1]))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	doc, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(s.Validate(doc))
}

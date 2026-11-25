// Najmniejszy program walidujący dokument według schematu: knroy/go-xml (przykład z README).
package main

import (
	"fmt"
	"os"

	"github.com/knroy/go-xml/v2/xdm"
	"github.com/knroy/go-xml/v2/xsd"
)

func main() {
	schema, err := xsd.LoadFile(os.Args[1], xsd.Options{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	d, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer d.Close()
	tree, err := xsd.ParseDocument(d, xdm.ParseOptions{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(schema.Validate(tree.Root, xsd.ValidateOptions{}))
}

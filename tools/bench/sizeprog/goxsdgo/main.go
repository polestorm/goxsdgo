// Najmniejszy program walidujący dokument według schematu: goxsdgo.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/polestorm/goxsdgo/xsd"
)

func main() {
	set, err := xsd.Compile(os.DirFS(filepath.Dir(os.Args[1])), filepath.Base(os.Args[1]))
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
	errs, err := set.Validate(d)
	fmt.Println(len(errs), err)
}

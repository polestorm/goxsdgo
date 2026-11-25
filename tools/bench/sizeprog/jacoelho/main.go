// Najmniejszy program walidujący dokument według schematu: jacoelho/xsd (przykład z README).
package main

import (
	"fmt"
	"os"

	"github.com/jacoelho/xsd"
)

func main() {
	engine, err := xsd.Compile(xsd.File(os.Args[1]))
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
	fmt.Println(engine.Validate(d))
}

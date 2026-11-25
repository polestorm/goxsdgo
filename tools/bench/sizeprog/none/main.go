// Program bazowy pomiaru rozmiaru: czyta oba pliki jak programy z walidatorem, ale niczego nie waliduje.
package main

import (
	"fmt"
	"os"
)

func main() {
	s, err := os.ReadFile(os.Args[1])
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
	fmt.Println(len(s))
}

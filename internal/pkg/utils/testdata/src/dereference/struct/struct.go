package pointer_to_struct

import "fmt"

type foo struct {
}

func f() {
	bar := foo{}
	fmt.Printf("%v", bar)
}

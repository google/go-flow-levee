package main

import (
	"fmt"
	"image"
)

func TestSingleBlock() {
	p := image.Point{1, 2}
	p.X = 3
	p.Y = 4
	fmt.Println(p.X + p.Y)
}

func TestMultiBlock() {
	p := image.Point{1, 2}
	if p.X > 0 {
		if p.Y > 0 {
			fmt.Printf("in top right quadrant, at (%d, %d)\n", p.X, p.Y)
		}
	} else {
		fmt.Println("somewhere")
	}
}

func TestParams(a, b int, c string) {
	fmt.Println(a, b, c)
}

func TestClosure() {
	x := 0
	f := func(x int) {
		fmt.Println(x)
	}
	f(x)
}

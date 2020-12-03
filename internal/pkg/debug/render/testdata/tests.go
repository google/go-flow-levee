// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func TestDisconnected() {
	x, y := 1, 2
	for i := 0; i < x*y; i++ {
		i--
	}

	prefix := "error: "
	message := "unreachable code"
	fmt.Printf(prefix + message)
}

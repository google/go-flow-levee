// Copyright 2020 Google LLC
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

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func testStaticFuncName(w io.Writer) {
	// positive cases
	var b bytes.Buffer
	b.Write(nil)            // want `^\Q(*bytes.Buffer).Write\E$`
	fmt.Println()           // want "^fmt.Println$"
	json.Unmarshal(nil, &b) // want "^encoding/json.Unmarshal$"

	// negative cases
	w.Write(nil) // want "^$"
	println()    // want "^$"
}

func testFuncNameWithoutReceiver(w io.Writer) {
	// positive cases
	var b bytes.Buffer
	b.Write(nil) // want "^Write$"
	w.Write(nil) // want "^Write$"

	// negative cases
	fmt.Println() // want "^$"
	println()     // want "^$"
}

func testFuncSignature(slice *[]*interface{}, m map[foo]string, r io.Reader) (err error, oops bool) { // want `^\Q(*[]*interface{},map[foo]string,Reader)(error,bool)\E$`
	return nil, false
}

func (f *foo) testMethodSignature(i int, d float64) *foo { // want `^\Q(int,float64)(*foo)\E$`
	return f
}

type foo struct{}

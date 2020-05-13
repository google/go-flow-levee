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

package allocation

import (
	"fmt"
)

type foo struct {
	name string
}

func f1() {
	f := &foo{name: "bar"}

	// Since f will exit ths scope of f1, log will not be added to the graph of f.
	// Expected graph:
	// new foo (complit) : *ssa.Alloc
	// &t0.name [#0] : *ssa.FieldAddr
	// *t1 = "bar":string : *ssa.Store

	log(f)
}

func log(in *foo) {
	fmt.Printf("Logging foo: %v", in)
}

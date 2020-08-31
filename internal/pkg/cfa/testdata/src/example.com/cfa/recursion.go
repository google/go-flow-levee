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

package cfa

import (
	"example.com/core"
)

func RecursiveSinkWrapper(i int, a interface{}) { // want RecursiveSinkWrapper:"genericFunc{ sinks: <1>, taints: <<> <>> }"
	if i <= 0 {
		core.Sink(a)
		return
	}
	RecursiveSinkWrapper(i-1, a)
}

func IsEven(i int) bool { // want IsEven:"genericFunc{ sinks: <>, taints: <<0>> }"
	return !IsOdd(i)
}

func IsOdd(i int) bool { // want IsOdd:"genericFunc{ sinks: <>, taints: <<0>> }"
	return !IsEven(i)
}

func A(e interface{}) { // want A:"genericFunc{ sinks: <0>, taints: <<>> }"
	B(e)
}

func B(e interface{}) { // want B:"genericFunc{ sinks: <0>, taints: <<>> }"
	C(e)
}

func C(e interface{}) { // want C:"genericFunc{ sinks: <0>, taints: <<>> }"
	if _, ok := e.(int); ok {
		A(0)
	} else {
		core.Sink(e)
	}
}

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
	"fmt"

	"example.com/core"
)

func OneParamSinkWrapper(a interface{}) { // want OneParamSinkWrapper:"genericFunc{ sinks: <0>, taints: <<>> }"
	core.Sink(a)
}

func TwoParamSinkWrapper(a interface{}, b interface{}) { // want TwoParamSinkWrapper:"genericFunc{ sinks: <0 1>, taints: <<> <>> }"
	core.Sink(a)
	core.Sink(b)
}

func OneParamSanitizedBeforeSinkCall(a interface{}) { // want OneParamSanitizedBeforeSinkCall:"genericFunc{ sinks: <>, taints: <<>> }"
	s := core.Sanitize(a)
	core.Sink(s)
}

func OneParamSanitizedBeforeReturn(a interface{}) interface{} { // want OneParamSanitizedBeforeReturn:"genericFunc{ sinks: <>, taints: <<>> }"
	s := core.Sanitize(a)
	return s
}

func OneParamTaintingOneResult(a interface{}) interface{} { // want OneParamTaintingOneResult:"genericFunc{ sinks: <>, taints: <<0>> }"
	return a
}

func OneParamTaintingBothResults(a interface{}) (interface{}, interface{}) { // want OneParamTaintingBothResults:"genericFunc{ sinks: <>, taints: <<0 1>> }"
	return a, a
}

func OneParamTaintingOneOfTwoResults(a interface{}) (interface{}, interface{}) { // want OneParamTaintingOneOfTwoResults:"genericFunc{ sinks: <>, taints: <<1>> }"
	return nil, a
}

func TwoParamsEachTaintingOneResult(a interface{}, b interface{}) (interface{}, interface{}) { // want TwoParamsEachTaintingOneResult:"genericFunc{ sinks: <>, taints: <<1> <0>> }"
	return b, a
}

func SinkWrapper(a interface{}, b interface{}) (interface{}, interface{}) { // want SinkWrapper:"genericFunc{ sinks: <0>, taints: <<> <0>> }"
	core.Sink(a)
	sanitized := core.Sanitize(a)
	tainted := []interface{}{b}
	return tainted, sanitized
}

func SinkWrapperWrapper(c interface{}) { // want SinkWrapperWrapper:"genericFunc{ sinks: <0>, taints: <<>> }"
	SinkWrapper(c, "")
}

func SinkWrapperWrapperWrapper(d interface{}, e interface{}) { // want SinkWrapperWrapperWrapper:"genericFunc{ sinks: <0 1>, taints: <<> <>> }"
	SinkWrapper(d, "d")
	SinkWrapper(e, "e")
}

func SinksFive(a interface{}) { // want SinksFive:"genericFunc{ sinks: <>, taints: <<>> }"
	five := ReturnsFive(a)
	core.Sink(five)
}

func ReturnsFive(a interface{}) int { // want ReturnsFive:"genericFunc{ sinks: <>, taints: <<>> }"
	return 5
}

func SinksThroughIdentity(a interface{}) { // want SinksThroughIdentity:"genericFunc{ sinks: <0>, taints: <<>> }"
	i := Identity(a)
	core.Sink(i)
}

func Identity(a interface{}) interface{} { // want Identity:"genericFunc{ sinks: <>, taints: <<0>> }"
	return a
}

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

func TestStringify(e interface{}) { // want TestStringify:"genericFunc{ sinks: <0>, taints: <<>> }"
	s := Stringify(e)
	core.Sink(s)
}

func Stringify(e interface{}) string { // want Stringify:"genericFunc{ sinks: <>, taints: <<0>> }"
	return fmt.Sprintf("%v", e)
}

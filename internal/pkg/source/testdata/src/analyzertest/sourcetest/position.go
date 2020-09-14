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

package sourcetest

func TestSourceValueDeclaration() {
	s := Source{} // want "source identified at .*position.go:18:2"
	_ = s
}

func TestSourcePointerDeclaration() {
	s := &Source{} // want "source identified at .*position.go:23:14"
	_ = s
}

func TestSourceValueParameter(val Source) { // want "source identified at .*position.go:27:31"
}

func TestSourcePointerParameter(ptr *Source) { // want "source identified at .*position.go:30:33"
}

func TestSourceValueExtract() {
	s, _ := CreateSource() // want "source identified at .*position.go:34:2"
	_ = s
}

func TestSourcePointerExtract() {
	s, _ := NewSource() // want "source identified at .*position.go:39:19"
	_ = s
}

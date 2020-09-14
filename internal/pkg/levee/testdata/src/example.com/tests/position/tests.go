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

package position

import (
	"example.com/core"
)

func TestSourcePointerExtract() {
	s, _ := NewSource()
	core.Sink(s) // want "a source has reached a sink, source: .*tests.go:22:19"
}

func NewSource() (*core.Source, error) {
	return &core.Source{}, nil
}

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

package source

type Source struct {
	secret  string `levee:"source"`
	data    string
	dataPtr *string
	id      int
}

func (s Source) ID() int {
	return s.id
}

func (s Source) Secret() string { // want Secret:"field propagator identified"
	return s.secret
}

func (s Source) DataValueReceiver() string { // want DataValueReceiver:"field propagator identified"
	return s.data
}

func (s *Source) DataPointerReceiver() string { // want DataPointerReceiver:"field propagator identified"
	return s.data
}

func (s Source) DataRef() *string { // want DataRef:"field propagator identified"
	return &s.data
}

func (s Source) DataPtr() *string { // want DataPtr:"field propagator identified"
	return s.dataPtr
}

func (s Source) DataDeref() string { // want DataDeref:"field propagator identified"
	return *s.dataPtr
}

func (s Source) ShowData() string { // want ShowData:"field propagator identified"
	return "Data: " + s.data
}

var isAdmin bool

func (s Source) MaybeData() string { // want MaybeData:"field propagator identified"
	if isAdmin {
		return s.data
	}
	return "<redacted>"
}

func (s Source) TryGetData() (string, error) { // want TryGetData:"field propagator identified"
	return s.data, nil
}

func New(data string) Source {
	return Source{data: data, id: 0}
}

type NotSource struct {
	data string
}

func (n NotSource) Data() string {
	return n.data
}

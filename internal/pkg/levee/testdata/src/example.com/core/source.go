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

package core

// Source will be configured to be detected as a source struct, with Source.Data as the source field.
// Source.Tagged is also a source field, as indicated by the field tag.
type Source struct {
	Data   string
	Tagged string `levee:"source"`
	ID     int
}

func (s Source) GetID() int {
	return s.ID
}

func (s Source) GetData() string {
	return s.Data
}

func (s Source) GetTagged() string {
	return s.Tagged
}

// Innocuous will _not_ be configured to be a source, even though underlying types are equal.
type Innocuous struct {
	Data string
	ID   int
}

func (i Innocuous) GetID() int {
	return i.ID
}

func (i Innocuous) GetData() string {
	return i.Data
}

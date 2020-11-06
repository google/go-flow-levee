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

package sourcetype

type Source struct { // want Source:"source type"
	Data         string // want Data:"source field"
	Tagged       string `levee:"source"` // want Tagged:"source field"
	Name1, Name2 string `levee:"source"` // want Name1:"source field" Name2:"source field"
	ID           int
}

type SourceContainingTaggedField struct { // want SourceContainingTaggedField:"source type"
	Tagged string `levee:"source"` // want Tagged:"source field"
}

type AliasStruct = Source // want AliasStruct:"source type"

type NamedType Source // TODO want NamedType:"source type"

type SliceContainer []Source
type ArrayContainer [5]Source
type MapKeyContainer map[Source]interface{}
type MapValueContainer map[string]Source

type EmbeddedWrapper struct {
	Source
}

type FieldWrapper struct {
	s Source
}

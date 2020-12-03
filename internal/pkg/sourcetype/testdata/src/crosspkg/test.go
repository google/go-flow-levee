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

package crosspkg

import "sourcetype"

type AliasStruct = sourcetype.Source // want AliasStruct:"source type"

type NamedType sourcetype.Source // want NamedType:"source type"

// TODO(96,97) Consider automatic detection of the following types.

type SliceContainer []sourcetype.Source
type ArrayContainer [5]sourcetype.Source
type MapKeyContainer map[sourcetype.Source]interface{}
type MapValueContainer map[string]sourcetype.Source

type EmbeddedWrapper struct {
	sourcetype.Source
}

type FieldWrapper struct {
	s sourcetype.Source
}

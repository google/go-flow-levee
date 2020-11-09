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

import (
	"example.com/source"
)

type (
	DefinedSource      source.Source                           // want DefinedSource:"inferred source"
	RedefinedSource    DefinedSource                           // want RedefinedSource:"inferred source"
	SourcePointer      *source.Source                          // want SourcePointer:"inferred source"
	SourceSlice        []source.Source                         // want SourceSlice:"inferred source"
	SourceMap          map[string]source.Source                // want SourceMap:"inferred source"
	SourceNested       map[string][]map[string][]source.Source // want SourceNested:"inferred source"
	DefinedSourceSlice []DefinedSource                         // want DefinedSourceSlice:"inferred source"
	DefinedFromTagged  source.Tagged                           // want DefinedFromTagged:"inferred source"
)

type (
	NotDefinedSource       source.NotSource
	FunctionsAreNotSources func() map[string]source.Source
)

func Typedef() {
	type DefinedInFunc source.Source // want DefinedInFunc:"inferred source"
}

func TypedefAnonymousFunc() {
	_ = func() {
		type DefinedInAnonymousFunc source.Source // want DefinedInAnonymousFunc:"inferred source"
	}
}

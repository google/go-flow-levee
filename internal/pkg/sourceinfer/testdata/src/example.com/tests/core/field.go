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
	SourceEmbedder struct { // want SourceEmbedder:"inferred source"
		source.Source
	}

	SourceHolder struct { // want SourceHolder:"inferred source"
		s source.Source
	}

	SourcePointerHolder struct { // want SourcePointerHolder:"inferred source"
		s *source.Source
	}

	SourceHolderAmongOtherThings struct { // want SourceHolderAmongOtherThings:"inferred source"
		s  source.Source
		ns source.NotSource
	}

	SourceSliceHolder struct { // want SourceSliceHolder:"inferred source"
		ss []source.Source
	}

	SourceNestedHolder struct { // want SourceNestedHolder:"inferred source"
		sn map[string][]map[string][]source.Source
	}

	SourceHolderHolder struct { // want SourceHolderHolder:"inferred source"
		sh SourceHolder
	}

	SourceWrapperHolder struct { // want SourceWrapperHolder:"inferred source"
		Wrapped struct {
			s source.Source
		}
	}

	TaggedWrapperHolder struct { // want TaggedWrapperHolder:"inferred source"
		Wrapped struct {
			s string `levee:"source"`
		}
	}

	TaggedHolder struct { // want TaggedHolder:"inferred source"
		t source.Tagged
	}
)

type (
	NotSourceHolder struct {
		ns source.NotSource
	}

	FieldFuncWithSourceArg struct {
		f func(s source.Source)
	}

	FieldFuncWithSourceRetval struct {
		f func() source.Source
	}
)

func Field() {
	type HoldingInFunc struct { // want HoldingInFunc:"inferred source"
		s source.Source
	}
}

func FieldAnonymousFunc() {
	_ = func() {
		type HoldingInAnonymousFunc struct { // want HoldingInAnonymousFunc:"inferred source"
			s source.Source
		}
	}
}

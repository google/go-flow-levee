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
	A struct { // want A:"inferred source"
		source.Source
		b *B
	}

	B struct { // want B:"inferred source"
		a *A
	}
)

type (
	C struct { // want C:"inferred source"
		source.Source
		d *D
	}

	D struct { // want D:"inferred source"
		e *E
	}

	E struct { // want E:"inferred source"
		c *C
	}
)

type (
	F struct {
		g *G
	}

	G struct {
		f *F
	}
)

type SelfRecursive struct {
	*SelfRecursive
}

type SelfRecursiveWithSource struct { // want SelfRecursiveWithSource:"inferred source"
	*SelfRecursiveWithSource
	s source.Source
}

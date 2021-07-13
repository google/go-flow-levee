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

// Package sourcetype handles identification of sources based on their type.
package sourcetype

import (
	"fmt"
	"go/types"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/utils"
)

// IsSourceType determines whether a Type is a Source Type.
// A Source Type is either:
// - A Named Struct Type that is configured as a Source
// - A Struct Type that contains a tagged field
// - A composite type that contains a Source Type
func IsSourceType(c *config.Config, tf fieldtags.ResultType, t types.Type) bool {
	seen := map[types.Type]bool{}
	return isSourceType(c, tf, t, seen)
}

// isSourceType is a helper method for IsSourceType.
// The set of seen types is kept track of to prevent infinite recursion on
// types such as `type A map[string]A`, which refer to themselves.
func isSourceType(c *config.Config, tf fieldtags.ResultType, t types.Type, seen map[types.Type]bool) bool {
	// If a type has been seen, then its status as a Source has already
	// been evaluated. Return to avoid infinite recursion.
	if seen[t] {
		return false
	}
	seen[t] = true

	switch tt := t.(type) {
	case *types.Named:
		return c.IsSourceType(utils.DecomposeType(tt)) || isSourceType(c, tf, tt.Underlying(), seen)
	case *types.Array:
		return isSourceType(c, tf, tt.Elem(), seen)
	case *types.Slice:
		return isSourceType(c, tf, tt.Elem(), seen)
	case *types.Chan:
		return isSourceType(c, tf, tt.Elem(), seen)
	case *types.Map:
		key := isSourceType(c, tf, tt.Key(), seen)
		elem := isSourceType(c, tf, tt.Elem(), seen)
		return key || elem
	case *types.Pointer:
		return isSourceType(c, tf, tt.Elem(), seen)
	case *types.Struct:
		return hasTaggedField(tf, tt)
	case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
		// These types do not currently represent possible source types
		return false
	default:
		// The above should be exhaustive.  Reaching this default case is an error.
		fmt.Printf("unexpected type received: %T %v; please report this issue\n", tt, tt)
		return false
	}
}

func hasTaggedField(taggedFields fieldtags.ResultType, s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if taggedFields.IsSource(f) {
			return true
		}
	}
	return false
}

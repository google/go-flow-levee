// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"testing"
)

func TestFieldTagsIdentification(t *testing.T) {
	if err := FlagSet.Set("config", "testdata/test-config.json"); err != nil {
		t.Error(err)
	}

	config, err := ReadConfig()
	if err != nil {
		t.Error(err)
	}

	cases := []struct {
		desc string
		key  string
		val  string
		want bool
	}{
		{
			"built-in field tag",
			"levee",
			"source",
			true,
		},
		{
			"custom field tag",
			"example",
			"sensitive",
			true,
		},
		{
			"different tag key",
			"notexample",
			"sensitive",
			false,
		},
		{
			"different tag value",
			"example",
			"notsensitive",
			false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			got := config.IsSourceFieldTag(tt.key, tt.val)
			if got != tt.want {
				t.Errorf("config.IsSourceFieldTag(%q, %q) == %v, want %v", tt.key, tt.val, got, tt.want)
			}
		})
	}
}

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

package config

import (
	"testing"
)

func TestFieldTagsIdentification(t *testing.T) {
	if err := FlagSet.Set("config", "testdata/test-config.yaml"); err != nil {
		t.Error(err)
	}

	config, err := ReadConfig()
	if err != nil {
		t.Error(err)
	}

	cases := []struct {
		desc string
		tag  string
		want bool
	}{
		{
			"built-in field tag",
			"`levee:\"source\"`",
			true,
		},
		{
			"custom field tag",
			"`example:\"sensitive\"`",
			true,
		},
		{
			"different tag key",
			"`notexample:\"sensitive\"`",
			false,
		},
		{
			"different tag value",
			"`example:\"notsensitive\"`",
			false,
		},
		{
			"escaped tag accepted",
			`"levee:\"source\""`,
			true,
		},
		{
			"multiple values, no target",
			"`example:\"foo,bar,baz\"`",
			false,
		},
		{
			"multiple values, with target",
			"`example:\"foo,sensitive,bar\"`",
			true,
		},
		{
			"multiple key value sets, no target",
			"`foo:\"bar,baz\" example:\"foo,bar,baz\" fizz:\"bang\"`",
			false,
		},
		{
			"multiple key value sets, with target",
			"`foo:\"bar,baz\" example:\"foo,sensitive,bar\"` fizz:\"bang\"",
			true,
		},
		{
			"empty",
			"",
			false,
		},
		{
			"malformed",
			"`noEndQuote:\"malform",
			false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			got := config.IsSourceFieldTag(tt.tag)
			if got != tt.want {
				t.Errorf("config.IsSourceFieldTag(%q) == %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

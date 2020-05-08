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

package regexp

import (
	"encoding/json"
	"testing"
)

func TestFlagParser(t *testing.T) {
	testCases := []struct {
		desc      string
		in        []byte
		wantMatch string
		wantErr   bool
	}{
		{
			desc:      "valid regex",
			in:        []byte(`"^hello$"`),
			wantMatch: "hello",
		},
		{
			desc:    "empty regex",
			in:      []byte(""),
			wantErr: true,
		},
		{
			desc:    "invalid regex",
			in:      []byte("["),
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			got := &Regexp{}
			err := json.Unmarshal(tt.in, &got)

			if tt.wantErr && err == nil {
				t.Fatalf("Got nil, wanted error %v", tt.wantErr)
			}

			if !got.MatchString(tt.wantMatch) {
				t.Fatalf("Got false, wanted true for got.MatchStrng(%s)", tt.wantMatch)
			}
		})
	}
}

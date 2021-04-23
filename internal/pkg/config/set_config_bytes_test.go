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

	"github.com/google/go-cmp/cmp"
)

func TestSetConfigBytes(t *testing.T) {
	set := &Config{ReportMessage: "test"}
	bytes := []byte(`ReportMessage: "test"`)

	SetBytes(bytes)

	read, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig returned an unexpected error: %v", err)
	}

	if diff := cmp.Diff(set, read); diff != "" {
		t.Errorf("set config differs from read config (-set, +read):\n%s", diff)
	}
}

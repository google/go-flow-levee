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

import (
	"config_analysistest/example/core"
	"config_analysistest/example/notcore"
	necore "config_analysistest/notexample/core"
)

func CoreCalls() {
	core.Sink() // want "sink call"
	core.NotSink()
	s := core.Sinker{}
	s.Do() // want "sink call"
	s.DoNot()
}

func NotCoreCalls() {
	notcore.Sink()
	notcore.NotSink()
	s := notcore.Sinker{}
	s.Do()
	s.DoNot()
}

func NotExampleComCalls() {
	necore.Sink()
	s := necore.Sinker{}
	s.Do()
}

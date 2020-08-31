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

func Sink() {} // want "sink"

func NotSink() {}

type Sinker struct{}

func (s Sinker) Do() {} // want "sink"

func (s Sinker) DoNot() {}

type NotSinker struct{}

func (ns NotSinker) Do() {}

func Calls() {
	Sink() // want "sink call"
	NotSink()
	s := Sinker{}
	s.Do() // want "sink call"
	s.DoNot()
	ns := NotSinker{}
	ns.Do()
}

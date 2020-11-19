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

func Excluded() {} // want "function Excluded is excluded from analysis"

func Sanitize() {} // want "function Sanitize is a sanitizer"

func Sink() {} // want "function Sink is a sink"

func SomeOtherFunction() {}

type Sinker struct{}

func (s Sinker) Do() {} // want "function \\(Sinker\\).Do is a sink"

func (s Sinker) DoNot() {}

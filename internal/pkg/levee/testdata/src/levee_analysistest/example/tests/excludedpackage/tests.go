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

package excludedpackage

import (
	"levee_analysistest/example/core"
)

func Oops(s core.Source) {
	core.Sink(s) // we do not expect a report here, because this package is excluded from analysis
}

func OopsIDidItAgain(s core.Source) {
	core.Sink(s) // we do not expect a report here, because this package is excluded from analysis
}

type Receiver int

func (Receiver) OopsWithReceiver(s core.Source) {
	core.Sink(s) // we do not expect a report here, because this package is excluded from analysis
}

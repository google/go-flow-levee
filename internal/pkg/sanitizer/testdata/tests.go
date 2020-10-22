// Copyright 2019 Google LLC
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

// Package dominance contains test-cases for testing PII sanitization.
package tests

import (
	"log"
	"time"
)

func dominatedSameBlock() {
	pwd := "password"
	pwd = scrub(pwd)
	log.Print(pwd) // want "dominated"
}

func notDominatedSameBlock() {
	pwd := "password"
	log.Print(pwd)
	pwd = scrub(pwd)
}

func dominatedDifferentBlocks() {
	pwd := "password"
	pwd = scrub(pwd)
	if time.Now().Weekday() == time.Monday {
		log.Print(pwd) // want "dominated"
	}
}

func notDominatedDifferentBlocks() {
	pwd := "P@ssword1"
	if time.Now().Weekday() == time.Monday {
		pwd = scrub(pwd)
		log.Print(pwd) // want "dominated"
	}

	log.Print(pwd)
}

func scrub(in string) string {
	return "scrubbed"
}

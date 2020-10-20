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

// package discussion demonstrates some examples of configuration intended by the new config style
package discussion

import "os"

// All Tokens are sources, even though the underlying type is a basic kind.
type Token string

// Since configuration does not specify any field as a source, then any TokenBundle is identified as a source.
// (Implicitly, every field is a source field.)
type TokenBundle struct {
	bundle                     []string
	masterToken                string
	internalCommunicationToken string
}

// Configuration specifically identifies a field as a source in this type.
// When a type has a source field, direct access of the non-source fields should not propagate taint.
type NamedToken struct {
	token string
	name  string
}

// Configuration specifically identifies a field as a source in this type via field tag.
// As above, direct access of non-source fields should not propagate taint.
type TaggedToken struct {
	Token string `myTag:"source"`
	name  string
}

func Foo() {
	// The string in this environment variable is sensitive.
	password := os.Getenv("CFSSL_CA_PK_PASSWORD")

	// The string in this environment variable is not.
	userDir := os.Getenv("USER_DIR")

	_, _ = password, userDir
}

func SaveSecret(secret interface{}) {
	// The parameter secret, though received by interface{}, is still identified as a source.
}

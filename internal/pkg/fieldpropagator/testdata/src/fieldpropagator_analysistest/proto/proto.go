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

package proto

type KeyPair struct {
	Public  []byte
	Private []byte
}

func (kp *KeyPair) GetPublic() []byte {
	if kp != nil {
		return kp.Public
	}
	return nil
}

func (kp *KeyPair) GetPrivate() []byte { // want GetPrivate:"field propagator identified"
	if kp != nil {
		return kp.Private
	}
	return nil
}

type Certificate struct{}

type TLS struct {
	Flag *bool
	Cert *Certificate
}

func (t *TLS) GetFlag() bool {
	if t != nil && t.Flag != nil {
		return *t.Flag
	}
	return false
}

func (t *TLS) GetCert() *Certificate { // want GetCert:"field propagator identified"
	if t != nil {
		return t.Cert
	}
	return nil
}

type BasicAuth struct {
	Username, Password *string
}

func (ba *BasicAuth) GetUsername() string {
	if ba != nil && ba.Username != nil {
		return *ba.Username
	}
	return ""
}

func (ba *BasicAuth) GetPassword() string { // want GetPassword:"field propagator identified"
	if ba != nil && ba.Password != nil {
		return *ba.Password
	}
	return ""
}

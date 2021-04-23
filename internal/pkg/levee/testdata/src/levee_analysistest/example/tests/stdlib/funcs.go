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

package stdlib

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"levee_analysistest/example/core"
	"strings"
	"sync"
	"text/template"
)

func TestTaintFromArgumentToReturnValue(s core.Source) {
	core.Sink(fmt.Errorf(s.Data))         // want "a source has reached a sink"
	core.Sink(strings.Split(s.Data, ",")) // want "a source has reached a sink"
}

func TestTaintFromArgumentToArgument(w io.Writer, s core.Source) {
	// w hasn't been tainted yet
	core.Sink(w)

	fmt.Fprintf(w, s.Data)
	core.Sink(w) // want "a source has reached a sink"
}

func TestTaintFromFourthArgumentToSecondArgument(t *template.Template, w io.Writer, s core.Source) {
	err := t.ExecuteTemplate(w, "template", s.Data)
	core.Sink(w) // want "a source has reached a sink"
	core.Sink(err)
}

func TestTaintFromFirstArgumentToReceiver(m *sync.Map, s core.Source) {
	m.Store(s.Data, nil)
	core.Sink(m) // want "a source has reached a sink"
}

func TestTaintFromSecondArgumentToReceiver(m *sync.Map, s core.Source) {
	m.Store(nil, s.Data)
	core.Sink(m) // want "a source has reached a sink"
}

func TestTaintFromArgumentToReceiver(scan bufio.Scanner, src core.Source) {
	scan.Buffer([]byte(src.Data), 1024)
	core.Sink(scan)        // TODO(#212) want "a source has reached a sink"
	core.Sink(scan.Text()) // TODO(#212) want "a source has reached a sink"
}

func TestTaintFromArgumentToPtrReceiver(scan *bufio.Scanner, src core.Source) {
	scan.Buffer([]byte(src.Data), 1024)
	core.Sink(scan)        // want "a source has reached a sink"
	core.Sink(scan.Text()) // want "a source has reached a sink"
}

func TestTaintFromReceiverToReturnValue(s core.Source) {
	b := bytes.NewBufferString(s.Data)
	line, err := b.ReadString(' ')
	core.Sink(line) // want "a source has reached a sink"
	core.Sink(err)
}

func TestTaintFromReceiverToArgument(str *string, src core.Source) {
	dec := json.NewDecoder(strings.NewReader(src.Data))
	dec.Decode(str)
	core.Sink(str) // want "a source has reached a sink"
}

func TestTaintFromVariadicToReturnValue(s core.Source) {
	core.Sink(fmt.Errorf("here's some data: %v", s.Data)) // want "a source has reached a sink"
}

func TestTaintFromMultiArgumentVariadicToReturnValue(s core.Source) {
	core.Sink(fmt.Errorf("source %d has data: %v", s.ID, s.Data)) // want "a source has reached a sink"
}

func TestTaintFromVariadicToArgument(w io.Writer, s core.Source) {
	fmt.Fprint(w, s.Data)
	core.Sink(w) // want "a source has reached a sink"
}

func TestTaintVariadic(str *string, src core.Source) {
	fmt.Sscan(src.Data, &str)
	core.Sink(str) // TODO(#291) want "a source has reached a sink"
}

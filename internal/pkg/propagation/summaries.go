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

package propagation

// A summary captures the behavior of a function with respect to taint
// propagation. Specifically, given that at least one of the necessary
// arguments is tainted, which arguments/return values become tainted?
// Note that when it's present, the receiver counts as an argument.
//
// As an example, consider fmt.Fprintf:
//   func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
// Its summary is:
//   "fmt.Fprintf": {
//   	ifTainted:   0b110,
//   	taintedArgs: []int{0},
//   },
// In English, this says that if the format string or the varargs slice are
// tainted, then the Writer is tainted.
type summary struct {
	// ifTainted is a bitset which contains positions for parameters
	// such that if one of these parameters is tainted, taint should
	// be propagated to the arguments and return values.
	// There is a 1-to-1 mapping between the bits and the function's
	// parameters, with the least significant bit corresponding to the
	// first (0th) argument.
	ifTainted int64
	// the positions of the arguments that taint propagates to if one of the
	// positions in ifTainted is tainted
	taintedArgs []int
	// the positions of the return values that taint propagates to if one of the
	// positions in ifTainted is tainted
	taintedRets []int
}

var fromFirstArgToFirstRet = summary{
	ifTainted:   0b1,
	taintedRets: []int{0},
}

// funcSummaries contains summaries for regular functions
// that could be called statically.
var funcSummaries = map[string]summary{
	// func Errorf(format string, a ...interface{}) error {
	"fmt.Errorf": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func Sprint(a ...interface{}) string {
	"fmt.Sprint": fromFirstArgToFirstRet,
	// func Sprintf(format string, a ...interface{}) string {
	"fmt.Sprintf": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func Sprintln(a ...interface{}) string {
	"fmt.Sprintln": fromFirstArgToFirstRet,
	// func Fprint(w io.Writer, a ...interface{}) (n int, err error) {
	"fmt.Fprint": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	"fmt.Fprintf": {
		ifTainted:   0b110,
		taintedArgs: []int{0},
	},
	// func Fprintln(w io.Writer, a ...interface{}) (n int, err error) {
	"fmt.Fprintln": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func Sscan(str string, a ...interface{}) (n int, err error) {
	"fmt.Sscan": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func Sscanln(str string, a ...interface{}) (n int, err error) {
	"fmt.Sscanln": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func Sscanf(str string, format string, a ...interface{}) (n int, err error) {
	"fmt.Sscanf": {
		ifTainted:   0b1,
		taintedArgs: []int{2},
	},
	// func Fscan(r io.Reader, a ...interface{}) (n int, err error) {
	"fmt.Fscan": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func Fscanln(r io.Reader, a ...interface{}) (n int, err error) {
	"fmt.Fscanln": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func Fscanf(r io.Reader, format string, a ...interface{}) (n int, err error) {
	"fmt.Fscanf": {
		ifTainted:   0b1,
		taintedArgs: []int{2},
	},
	// func New(text string) error {
	"errors.New": fromFirstArgToFirstRet,
	// func Unwrap(err error) error {
	"errors.Unwrap": fromFirstArgToFirstRet,
	// func As(err error, target interface{}) bool {
	"errors.As": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func SplitN(s, sep string, n int) []string {
	"strings.SplitN": fromFirstArgToFirstRet,
	// func SplitAfterN(s, sep string, n int) []string {
	"strings.SplitAfterN": fromFirstArgToFirstRet,
	// func Split(s, sep string) []string {
	"strings.Split": fromFirstArgToFirstRet,
	// func SplitAfter(s, sep string) []string {
	"strings.SplitAfter": fromFirstArgToFirstRet,
	// func Fields(s string) []string {
	"strings.Fields": fromFirstArgToFirstRet,
	// func FieldsFunc(s string, f func(rune) bool) []string {
	"strings.FieldsFunc": fromFirstArgToFirstRet,
	// func Join(elems []string, sep string) string {
	"strings.Join": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func Map(mapping func(rune) rune, s string) string {
	"strings.Map": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func Repeat(s string, count int) string {
	"strings.Repeat": fromFirstArgToFirstRet,
	// func ToUpper(s string) string {
	"strings.ToUpper": fromFirstArgToFirstRet,
	// func ToLower(s string) string {
	"strings.ToLower": fromFirstArgToFirstRet,
	// func ToTitle(s string) string {
	"strings.ToTitle": fromFirstArgToFirstRet,
	// func ToUpperSpecial(c unicode.SpecialCase, s string) string {
	"strings.ToUpperSpecial": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func ToLowerSpecial(c unicode.SpecialCase, s string) string {
	"strings.ToLowerSpecial": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func ToTitleSpecial(c unicode.SpecialCase, s string) string {
	"strings.ToTitleSpecial": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func ToValidUTF8(s, replacement string) string {
	"strings.ToValidUTF8": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func Title(s string) string {
	"strings.Title": fromFirstArgToFirstRet,
	// func TrimLeftFunc(s string, f func(rune) bool) string {
	"strings.TrimLeftFunc": fromFirstArgToFirstRet,
	// func TrimRightFunc(s string, f func(rune) bool) string {
	"strings.TrimRightFunc": fromFirstArgToFirstRet,
	// func TrimFunc(s string, f func(rune) bool) string {
	"strings.TrimFunc": fromFirstArgToFirstRet,
	// func Trim(s, cutset string) string {
	"strings.Trim": fromFirstArgToFirstRet,
	// func TrimLeft(s, cutset string) string {
	"strings.TrimLeft": fromFirstArgToFirstRet,
	// func TrimRight(s, cutset string) string {
	"strings.TrimRight": fromFirstArgToFirstRet,
	// func TrimSpace(s string) string {
	"strings.TrimSpace": fromFirstArgToFirstRet,
	// func TrimPrefix(s, prefix string) string {
	"strings.TrimPrefix": fromFirstArgToFirstRet,
	// func TrimSuffix(s, suffix string) string {
	"strings.TrimSuffix": fromFirstArgToFirstRet,
	// func Replace(s, old, new string, n int) string {
	"strings.Replace": {
		ifTainted:   0b101,
		taintedRets: []int{0},
	},
	// func ReplaceAll(s, old, new string) string {
	"strings.ReplaceAll": {
		ifTainted:   0b101,
		taintedRets: []int{0},
	},
	// func NewReader(s string) *Reader {
	"strings.NewReader": fromFirstArgToFirstRet,
	// func (r *Replacer) Replace(s string) string {
	"(*strings.Replacer).Replace": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func (r *Replacer) WriteString(w io.Writer, s string) (n int, err error) {
	"(*strings.Replacer).WriteString": {
		ifTainted:   0b101,
		taintedArgs: []int{1},
	},
	// func NewReplacer(oldnew ...string) *Replacer {
	"strings.NewReplacer": fromFirstArgToFirstRet,
	// func (b *Buffer) Next(n int) []byte {
	"(*bytes.Buffer).Next": fromFirstArgToFirstRet,
	// func (b *Buffer) ReadBytes(delim byte) (line []byte, err error) {
	"(*bytes.Buffer).ReadBytes": fromFirstArgToFirstRet,
	// func (b *Buffer) ReadString(delim byte) (line string, err error) {
	"(*bytes.Buffer).ReadString": fromFirstArgToFirstRet,
	// func NewBuffer(buf []byte) *Buffer {
	"bytes.NewBuffer": fromFirstArgToFirstRet,
	// func NewBufferString(s string) *Buffer {
	"bytes.NewBufferString": fromFirstArgToFirstRet,
	// func SplitN(s, sep []byte, n int) [][]byte {
	"bytes.SplitN": fromFirstArgToFirstRet,
	// func SplitAfterN(s, sep []byte, n int) [][]byte {
	"bytes.SplitAfterN": fromFirstArgToFirstRet,
	// func Split(s, sep []byte) [][]byte {
	"bytes.Split": fromFirstArgToFirstRet,
	// func SplitAfter(s, sep []byte) [][]byte {
	"bytes.SplitAfter": fromFirstArgToFirstRet,
	// func Fields(s []byte) [][]byte {
	"bytes.Fields": fromFirstArgToFirstRet,
	// func FieldsFunc(s []byte, f func(rune) bool) [][]byte {
	"bytes.FieldsFunc": fromFirstArgToFirstRet,
	// func Join(s [][]byte, sep []byte) []byte {
	"bytes.Join": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func Map(mapping func(r rune) rune, s []byte) []byte {
	"bytes.Map": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func Repeat(b []byte, count int) []byte {
	"bytes.Repeat": fromFirstArgToFirstRet,
	// func ToUpper(s []byte) []byte {
	"bytes.ToUpper": fromFirstArgToFirstRet,
	// func ToLower(s []byte) []byte {
	"bytes.ToLower": fromFirstArgToFirstRet,
	// func ToTitle(s []byte) []byte {
	"bytes.ToTitle": fromFirstArgToFirstRet,
	// func ToUpperSpecial(c unicode.SpecialCase, s []byte) []byte {
	"bytes.ToUpperSpecial": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func ToLowerSpecial(c unicode.SpecialCase, s []byte) []byte {
	"bytes.ToLowerSpecial": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func ToTitleSpecial(c unicode.SpecialCase, s []byte) []byte {
	"bytes.ToTitleSpecial": {
		ifTainted:   0b10,
		taintedRets: []int{0},
	},
	// func ToValidUTF8(s, replacement []byte) []byte {
	"bytes.ToValidUTF8": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func Title(s []byte) []byte {
	"bytes.Title": fromFirstArgToFirstRet,
	// func TrimLeftFunc(s []byte, f func(r rune) bool) []byte {
	"bytes.TrimLeftFunc": fromFirstArgToFirstRet,
	// func TrimRightFunc(s []byte, f func(r rune) bool) []byte {
	"bytes.TrimRightFunc": fromFirstArgToFirstRet,
	// func TrimFunc(s []byte, f func(r rune) bool) []byte {
	"bytes.TrimFunc": fromFirstArgToFirstRet,
	// func TrimPrefix(s, prefix []byte) []byte {
	"bytes.TrimPrefix": fromFirstArgToFirstRet,
	// func TrimSuffix(s, suffix []byte) []byte {
	"bytes.TrimSuffix": fromFirstArgToFirstRet,
	// func Trim(s []byte, cutset string) []byte {
	"bytes.Trim": fromFirstArgToFirstRet,
	// func TrimLeft(s []byte, cutset string) []byte {
	"bytes.TrimLeft": fromFirstArgToFirstRet,
	// func TrimRight(s []byte, cutset string) []byte {
	"bytes.TrimRight": fromFirstArgToFirstRet,
	// func TrimSpace(s []byte) []byte {
	"bytes.TrimSpace": fromFirstArgToFirstRet,
	// func Runes(s []byte) []rune {
	"bytes.Runes": fromFirstArgToFirstRet,
	// func Replace(s, old, new []byte, n int) []byte {
	"bytes.Replace": {
		ifTainted:   0b101,
		taintedRets: []int{0},
	},
	// func ReplaceAll(s, old, new []byte) []byte {
	"bytes.ReplaceAll": {
		ifTainted:   0b101,
		taintedRets: []int{0},
	},
	// func NewReader(b []byte) *Reader {
	"bytes.NewReader": fromFirstArgToFirstRet,
	// func WriteString(w Writer, s string) (n int, err error) {
	"io.WriteString": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error) {
	"io.ReadAtLeast": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func ReadFull(r Reader, buf []byte) (n int, err error) {
	"io.ReadFull": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func CopyN(dst Writer, src Reader, n int64) (written int64, err error) {
	"io.CopyN": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func Copy(dst Writer, src Reader) (written int64, err error) {
	"io.Copy": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func CopyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
	"io.CopyBuffer": {
		ifTainted:   0b10,
		taintedArgs: []int{0, 2},
	},
	// func LimitReader(r Reader, n int64) Reader {
	"io.LimitReader": fromFirstArgToFirstRet,
	// func TeeReader(r Reader, w Writer) Reader {
	"io.TeeReader": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func MultiReader(readers ...Reader) Reader {
	"io.MultiReader": fromFirstArgToFirstRet,
	// func MultiWriter(writers ...Writer) Writer {
	"io.MultiWriter": fromFirstArgToFirstRet,
	// func (r *PipeReader) CloseWithError(err error) error {
	"(*io.PipeReader).CloseWithError": fromFirstArgToFirstRet,
	// func (w *PipeWriter) CloseWithError(err error) error {
	"(*io.PipeWriter).CloseWithError": fromFirstArgToFirstRet,
	// func ReadAll(r io.Reader) ([]byte, error) {
	"io/ioutil.ReadAll": fromFirstArgToFirstRet,
	// func NopCloser(r io.Reader) io.ReadCloser {
	"io/ioutil.NopCloser": fromFirstArgToFirstRet,
	// func NewReaderSize(rd io.Reader, size int) *Reader {
	"bufio.NewReaderSize": fromFirstArgToFirstRet,
	// func NewReader(rd io.Reader) *Reader {
	"bufio.NewReader": fromFirstArgToFirstRet,
	// TODO: test
	// func (b *Reader) Peek(n int) ([]byte, error) {
	"(*bufio.Reader).Peek": fromFirstArgToFirstRet,
	// func (b *Reader) ReadSlice(delim byte) (line []byte, err error) {
	"(*bufio.Reader).ReadSlice": fromFirstArgToFirstRet,
	// func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error) {
	"(*bufio.Reader).ReadLine": fromFirstArgToFirstRet,
	// func (b *Reader) ReadBytes(delim byte) ([]byte, error) {
	"(*bufio.Reader).ReadBytes": fromFirstArgToFirstRet,
	// func (b *Reader) ReadString(delim byte) (string, error) {
	"(*bufio.Reader).ReadString": fromFirstArgToFirstRet,
	// func NewWriterSize(w io.Writer, size int) *Writer {
	"bufio.NewWriterSize": fromFirstArgToFirstRet,
	// func NewWriter(w io.Writer) *Writer {
	"bufio.NewWriter": fromFirstArgToFirstRet,
	// func NewReadWriter(r *Reader, w *Writer) *ReadWriter {
	"bufio.NewReadWriter": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func NewScanner(r io.Reader) *Scanner {
	"bufio.NewScanner": fromFirstArgToFirstRet,
	// func (s *Scanner) Bytes() []byte {
	"(*bufio.Scanner).Bytes": fromFirstArgToFirstRet,
	// func (s *Scanner) Text() string {
	"(*bufio.Scanner).Text": fromFirstArgToFirstRet,
	// func (s *Scanner) Buffer(buf []byte, max int) {
	"(*bufio.Scanner).Buffer": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func ScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	"bufio.ScanLines": {
		ifTainted:   0b1,
		taintedRets: []int{1},
	},
	// func ScanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	"bufio.ScanWords": {
		ifTainted:   0b1,
		taintedRets: []int{1},
	},
	// func WithValue(parent Context, key, val interface{}) Context {
	"context.WithValue": {
		ifTainted:   0b111,
		taintedRets: []int{0},
	},
	// func AppendBool(dst []byte, b bool) []byte {
	"strconv.AppendBool": fromFirstArgToFirstRet,
	// func AppendFloat(dst []byte, f float64, fmt byte, prec, bitSize int) []byte {
	"strconv.AppendFloat": fromFirstArgToFirstRet,
	// func AppendInt(dst []byte, i int64, base int) []byte {
	"strconv.AppendInt": fromFirstArgToFirstRet,
	// func AppendUint(dst []byte, i uint64, base int) []byte {
	"strconv.AppendUint": fromFirstArgToFirstRet,
	// func Quote(s string) string {
	"strconv.Quote": fromFirstArgToFirstRet,
	// func AppendQuote(dst []byte, s string) []byte {
	"strconv.AppendQuote": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func QuoteToASCII(s string) string {
	"strconv.QuoteToASCII": fromFirstArgToFirstRet,
	// func AppendQuoteToASCII(dst []byte, s string) []byte {
	"strconv.AppendQuoteToASCII": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func QuoteToGraphic(s string) string {
	"strconv.QuoteToGraphic": fromFirstArgToFirstRet,
	// func AppendQuoteToGraphic(dst []byte, s string) []byte {
	"strconv.AppendQuoteToGraphic": {
		ifTainted:   0b11,
		taintedRets: []int{0},
	},
	// func AppendQuoteRune(dst []byte, r rune) []byte {
	"strconv.AppendQuoteRune": fromFirstArgToFirstRet,
	// func AppendQuoteRuneToASCII(dst []byte, r rune) []byte {
	"strconv.AppendQuoteRuneToASCII": fromFirstArgToFirstRet,
	// func AppendQuoteRuneToGraphic(dst []byte, r rune) []byte {
	"strconv.AppendQuoteRuneToGraphic": fromFirstArgToFirstRet,
	// func UnquoteChar(s string, quote byte) (value rune, multibyte bool, tail string, err error) {
	"strconv.UnquoteChar": {
		ifTainted:   0b1,
		taintedRets: []int{2},
	},
	// func Unquote(s string) (string, error) {
	"strconv.Unquote": fromFirstArgToFirstRet,
	// func Unmarshal(data []byte, v interface{}) error {
	"encoding/json.Unmarshal": {
		ifTainted:   0b11,
		taintedArgs: []int{0, 1},
	},
	// func Marshal(v interface{}) ([]byte, error) {
	"encoding/json.Marshal": fromFirstArgToFirstRet,
	// func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	"encoding/json.MarshalIndent": fromFirstArgToFirstRet,
	// func HTMLEscape(dst *bytes.Buffer, src []byte) {
	"encoding/json.HTMLEscape": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func Compact(dst *bytes.Buffer, src []byte) error {
	"encoding/json.Compact": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	"encoding/json.Indent": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func NewDecoder(r io.Reader) *Decoder {
	"encoding/json.NewDecoder": fromFirstArgToFirstRet,
	// func (dec *Decoder) Decode(v interface{}) error {
	"(*encoding/json.Decoder).Decode": {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// func (dec *Decoder) Buffered() io.Reader {
	"(*encoding/json.Decoder).Buffered": fromFirstArgToFirstRet,
	// func (dec *Decoder) Token() (Token, error) {
	"(*encoding/json.Decoder).Token": fromFirstArgToFirstRet,
	// func NewEncoder(w io.Writer) *Encoder {
	"encoding/json.NewEncoder": fromFirstArgToFirstRet,
	// func (enc *Encoder) Encode(v interface{}) error {
	"(*encoding/json.Encoder).Encode": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func (m RawMessage) MarshalJSON() ([]byte, error) {
	"(encoding/json.RawMessage).MarshalJSON": fromFirstArgToFirstRet,
	// func (m *RawMessage) UnmarshalJSON(data []byte) error {
	"(*encoding/json.RawMessage).UnmarshalJSON": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func (enc *Encoding) Encode(dst, src []byte) {
	"(*encoding/base64.Encoding).Encode": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func (enc *Encoding) EncodeToString(src []byte) string {
	"(*encoding/base64.Encoding).EncodeToString": fromFirstArgToFirstRet,
	// func (enc *Encoding) DecodeString(s string) ([]byte, error) {
	"(*encoding/base64.Encoding).DecodeString": fromFirstArgToFirstRet,
	// func (enc *Encoding) Decode(dst, src []byte) (n int, err error) {
	"(*encoding/base64.Encoding).Decode": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func NewDecoder(enc *Encoding, r io.Reader) io.Reader {
	"encoding/base64.NewDecoder": fromFirstArgToFirstRet,
	// func (m *Map) Load(key interface{}) (value interface{}, ok bool) {
	"(*sync.Map).Load": fromFirstArgToFirstRet,
	// func (m *Map) Store(key, value interface{}) {
	"(*sync.Map).Store": {
		ifTainted:   0b110,
		taintedArgs: []int{0},
	},
	// func (m *Map) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	"(*sync.Map).LoadOrStore": {
		ifTainted:   0b111,
		taintedArgs: []int{0},
		taintedRets: []int{0},
	},
	// func (m *Map) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	"(*sync.Map).LoadAndDelete": fromFirstArgToFirstRet,
	// func (p *Pool) Put(x interface{}) {
	"(*sync.Pool).Put": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func (p *Pool) Get() interface{} {
	"(*sync.Pool).Get": fromFirstArgToFirstRet,
	// func (s *Scanner) Init(src io.Reader) *Scanner {
	"(*text/scanner.Scanner).Init": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
		taintedRets: []int{0},
	},
	// func (s *Scanner) TokenText() string {
	"(*text/scanner.Scanner).TokenText": fromFirstArgToFirstRet,
	// func (b *Writer) Write(buf []byte) (n int, err error) {
	"(*text/tabwriter.Writer).Write": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer {
	"text/tabwriter.NewWriter": fromFirstArgToFirstRet,
	// func (t *Template) ExecuteTemplate(wr io.Writer, name string, data interface{}) error {
	"(*text/template.Template).ExecuteTemplate": {
		ifTainted:   0b1000,
		taintedArgs: []int{1},
	},
	// func (t *Template) Execute(wr io.Writer, data interface{}) error {
	"(*text/template.Template).Execute": {
		ifTainted:   0b100,
		taintedArgs: []int{1},
	},
	// func HTMLEscape(w io.Writer, b []byte) {
	"text/template.HTMLEscape": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func HTMLEscapeString(s string) string {
	"text/template.HTMLEscapeString": fromFirstArgToFirstRet,
	// func HTMLEscaper(args ...interface{}) string {
	"text/template.HTMLEscaper": fromFirstArgToFirstRet,
	// func JSEscape(w io.Writer, b []byte) {
	"text/template.JSEscape": {
		ifTainted:   010,
		taintedArgs: []int{0},
	},
	// func JSEscapeString(s string) string {
	"text/template.JSEscapeString": fromFirstArgToFirstRet,
	// func JSEscaper(args ...interface{}) string {
	"text/template.JSEscaper": fromFirstArgToFirstRet,
	// func URLQueryEscaper(args ...interface{}) string {
	"text/template.URLQueryEscaper": fromFirstArgToFirstRet,
	// func (t *Template) ExecuteTemplate(wr io.Writer, name string, data interface{}) error {
	"(*html/template.Template).ExecuteTemplate": {
		ifTainted:   0b1000,
		taintedArgs: []int{1},
	},
	// func (t *Template) Execute(wr io.Writer, data interface{}) error {
	"(*html/template.Template).Execute": {
		ifTainted:   0b100,
		taintedArgs: []int{1},
	},
	// func HTMLEscape(w io.Writer, b []byte) {
	"html/template.HTMLEscape": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func HTMLEscapeString(s string) string {
	"html/template.HTMLEscapeString": fromFirstArgToFirstRet,
	// func HTMLEscaper(args ...interface{}) string {
	"html/template.HTMLEscaper": fromFirstArgToFirstRet,
	// func JSEscape(w io.Writer, b []byte) {
	"html/template.JSEscape": {
		ifTainted:   010,
		taintedArgs: []int{0},
	},
	// func JSEscapeString(s string) string {
	"html/template.JSEscapeString": fromFirstArgToFirstRet,
	// func JSEscaper(args ...interface{}) string {
	"html/template.JSEscaper": fromFirstArgToFirstRet,
	// func URLQueryEscaper(args ...interface{}) string {
	"html/template.URLQueryEscaper": fromFirstArgToFirstRet,
	// func Clean(path string) string {
	"path.Clean": fromFirstArgToFirstRet,
	// func Split(path string) (dir, file string) {
	"path.Split": {
		ifTainted:   0b1,
		taintedRets: []int{0, 1},
	},
	// func Join(elem ...string) string {
	"path.Join": fromFirstArgToFirstRet,
	// func Base(path string) string {
	"path.Base": fromFirstArgToFirstRet,
	// func Clean(path string) string {
	"path/filepath.Clean": fromFirstArgToFirstRet,
	// func ToSlash(path string) string {
	"path/filepath.ToSlash": fromFirstArgToFirstRet,
	// func FromSlash(path string) string {
	"path/filepath.FromSlash": fromFirstArgToFirstRet,
	// func SplitList(path string) []string {
	"path/filepath.SplitList": fromFirstArgToFirstRet,
	// func Split(path string) (dir, file string) {
	"path/filepath.Split": {
		ifTainted:   0b1,
		taintedRets: []int{0, 1},
	},
	// func Join(elem ...string) string {
	"path/filepath.Join": fromFirstArgToFirstRet,
	// func Ext(path string) string {
	"path/filepath.Ext": fromFirstArgToFirstRet,
	// func Abs(path string) (string, error) {
	"path/filepath.Abs": fromFirstArgToFirstRet,
	// func Base(path string) string {
	"path/filepath.Base": fromFirstArgToFirstRet,
	// func New(out io.Writer, prefix string, flag int) *Logger {
	"log.New": fromFirstArgToFirstRet,
	// func (l *Logger) SetOutput(w io.Writer) {
	"(*log.Logger).SetOutput": {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// func (l *Logger) Writer() io.Writer {
	"(*log.Logger).Writer": fromFirstArgToFirstRet,
}

// funcKey represents an interface function by its name and its signature.
// The signature is a string representation containing only the types of
// the arguments and return values.
type funcKey struct {
	name, signature string
}

// interfaceFuncSummaries contains summaries for common interface functions
// such as Write or Read, that could be called statically (i.e. a call to
// a concrete method whose signature matches an interface method) or dynamically
// (i.e. a call to an interface method on an interface value).
var interfaceFuncSummaries = map[funcKey]summary{
	// type io.Reader interface {
	//	Read(p []byte) (n int, err error)
	// }
	{"Read", "([]byte)(int,error)"}: {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// type io.Writer interface {
	//  Write(p []byte) (n int, err error)
	// }
	{"Write", "([]byte)(int,error)"}: {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// type io.ReaderFrom interface {
	// 	ReadFrom(r Reader) (n int64, err error)
	// }
	{"ReadFrom", "(Reader)(int64,error)"}: {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// type io.WriterTo interface {
	//  WriteTo(w Writer) (n int64, err error)
	// }
	{"WriteTo", "(Writer)(int64,error)"}: {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// type io.ReaderAt interface {
	//	ReadAt(p []byte, off int64) (n int, err error)
	// }
	{"ReadAt", "([]byte,int64)(int,error)"}: {
		ifTainted:   0b1,
		taintedArgs: []int{1},
	},
	// type io.WriterAt interface {
	//	WriteAt(p []byte, off int64) (n int, err error)
	// }
	{"WriteAt", "([]byte,int64)(int,error)"}: {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// type io.StringWriter interface {
	//	WriteString(s string) (n int, err error)
	// }
	{"WriteString", "(string)(int,error)"}: {
		ifTainted:   0b10,
		taintedArgs: []int{0},
	},
	// type fmt.Stringer interface {
	//	String() string
	// }
	{"String", "()(string)"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
	// type fmt.GoStringer interface {
	//	GoString() string
	// }
	{"String", "()(string)"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
	// type error interface {
	//	Error() string
	// }
	{"Error", "()(string)"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
	// Unwrap() error
	{"Unwrap", "()(error)"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
	// Bytes() []byte
	{"Bytes", "()([]byte)"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
	// type context.Context interface {
	//  Err() error
	//	Value(key interface{}) interface{}
	// }
	{"Err", "()(error)"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
	{"Value", "(interface{})(interface{})"}: {
		ifTainted:   0b1,
		taintedRets: []int{0},
	},
}

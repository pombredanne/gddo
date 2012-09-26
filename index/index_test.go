// Copyright 2012 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package index

import (
	"github.com/garyburd/gopkgdoc/doc"
	"reflect"
	"testing"
)

var addTests = []struct {
	a        postingList
	v        posting
	expected postingList
}{
	{postingList{{1}}, posting{1}, postingList{{1}}},
	{postingList{{1}}, posting{3}, postingList{{1}, {3}}},
	{postingList{{1}, {3}}, posting{2}, postingList{{1}, {2}, {3}}},
	{postingList{{1}, {2}, {3}}, posting{2}, postingList{{1}, {2}, {3}}},
}

var removeTests = []struct {
	a        postingList
	v        posting
	expected postingList
}{
	{postingList{{1}}, posting{1}, postingList{}},
	{postingList{{1}, {3}}, posting{3}, postingList{{1}}},
	{postingList{{1}, {2}, {3}}, posting{2}, postingList{{1}, {3}}},
	{postingList{{1}, {3}}, posting{2}, postingList{{1}, {3}}},
}

var intersectTests = []struct {
	a, b, expected postingList
}{
	{postingList{{1}}, postingList{{1}}, postingList{{1}}},
	{postingList{{1}}, postingList{{2}}, postingList{}},
	{postingList{{2}}, postingList{{1}}, postingList{}},
	{postingList{{1}, {2}}, postingList{{2}, {3}}, postingList{{2}}},
}

func TestPostingLists(t *testing.T) {
	for _, tt := range addTests {
		a := append(postingList(nil), tt.a...)
		actual := a.add(tt.v)
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Errorf("%v.add(%d) = %v, want %v", tt.a, tt.v, actual, tt.expected)
		}
	}

	for _, tt := range removeTests {
		a := append(postingList(nil), tt.a...)
		actual := a.remove(tt.v)
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Errorf("%v.add(%d) = %v, want %v", tt.a, tt.v, actual, tt.expected)
		}
	}

	for _, tt := range intersectTests {
		actual := tt.a.intersect(postingList{}, tt.b)
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Errorf("%v.intersect(%v) = %v, want %v", tt.a, tt.b, actual, tt.expected)
		}
	}
}

var testPkgs = []*doc.Package{
	{
		ImportPath:  "strconv",
		ProjectRoot: "",
		ProjectName: "Go",
		Name:        "strconv",
		Synopsis:    "Package strconv implements conversions to and from string representations of basic data types.",
		Doc:         "Package strconv implements conversions to and from string representations\nof basic data types.",
		Imports:     []string{"errors", "math", "unicode/utf8"},
		Funcs:       []*doc.Func{{}},
	},
	{
		ImportPath:  "github.com/garyburd/go-oauth/oauth",
		ProjectRoot: "github.com/garyburd/go-oauth",
		ProjectName: "go-oauth",
		ProjectURL:  "https://github.com/garyburd/go-oauth/",
		Name:        "oauth",
		Synopsis:    "Package oauth implements a subset of the OAuth client interface as defined in RFC 5849.",
		Doc: "Package oauth implements a subset of the OAuth client interface as defined in RFC 5849.\n\n" +
			"This package assumes that the application writes request URL paths to the\nnetwork using " +
			"the encoding implemented by the net/url URL RequestURI method.\n" +
			"The HTTP client in the standard net/http package uses this encoding.",
		IsCmd: false,
		Imports: []string{
			"bytes",
			"crypto/hmac",
			"crypto/rand",
			"crypto/sha1",
			"encoding/base64",
			"encoding/binary",
			"errors",
			"fmt",
			"io",
			"io/ioutil",
			"net/http",
			"net/url",
			"regexp",
			"sort",
			"strconv",
			"strings",
			"sync",
			"time",
		},
		TestImports: []string{"bytes", "net/url", "testing"},
		Funcs:       []*doc.Func{{}},
	},
	{
		// empty directory
		ImportPath:  "example.com/src",
		ProjectRoot: "example.com",
		ProjectName: "example",
		Name:        "",
	},
	{
		ImportPath:  "example.com/src/a",
		ProjectRoot: "example.com",
		ProjectName: "example",
		Name:        "a",
		Funcs:       []*doc.Func{{}},
	},
	{
		ImportPath:  "example.com/src/b",
		ProjectRoot: "example.com",
		ProjectName: "example",
		Name:        "b",
		Funcs:       []*doc.Func{{}},
	},
	{
		ImportPath:  "github.com/example/noexports",
		ProjectRoot: "github.com/exmaple/noexports",
		ProjectName: "noexports",
		Name:        "noexports",
	},
}

var testQueries = []struct {
	q        string
	expected []string
}{
	{"strconv", []string{"strconv"}},
	{"project:", []string{"strconv"}},
	{"project:github.com/garyburd/go-oauth", []string{"github.com/garyburd/go-oauth/oauth"}},
	{"import:bytes", []string{"github.com/garyburd/go-oauth/oauth"}},
	{"oauth", []string{"github.com/garyburd/go-oauth/oauth"}},
	{"all:", []string{"example.com/src/a", "example.com/src/b", "github.com/example/noexports", "github.com/garyburd/go-oauth/oauth", "strconv"}},
}

var testChildren = []struct {
	importPath string
	expected   []string
}{
	{"example.com", []string{"example.com/src/a", "example.com/src/b"}},
	{"notfound.com", []string{}},
	{"notfound.com/path", []string{}},
}

func TestIndex(t *testing.T) {
	idx := New()

	// Put

	for _, pkg := range testPkgs {
		if err := idx.Put(pkg); err != nil {
			t.Errorf("idx.Put(%s) -> %v", pkg.ImportPath, err)
		}
	}

	// Get

	for _, pkg := range testPkgs {
		actualPkg, err := idx.Get(pkg.ImportPath)
		if err != nil {
			t.Errorf("idx.Get(%q) -> %v", pkg.ImportPath, err)
			continue
		}
		if !reflect.DeepEqual(pkg, actualPkg) {
			t.Errorf("idx.Get(%q) = %+v, want %+v", pkg.ImportPath, actualPkg, pkg)
		}
	}

	// Query

	for _, tt := range testQueries {
		results, err := idx.Query(tt.q, SortByPath)
		if err != nil {
			t.Errorf("idx.Query(%q) -> %v", tt.q, err)
			continue
		}
		actual := make([]string, len(results))
		for i, result := range results {
			actual[i] = result.ImportPath
		}
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Errorf("idx.Query(%q) = %#v, want %#v", tt.q, actual, tt.expected)
		}
	}

	// Children

	for _, tt := range testChildren {
		results, _ := idx.Subdirs(tt.importPath)
		actual := make([]string, len(results))
		for i, result := range results {
			actual[i] = result.ImportPath
		}
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Errorf("idx.Children(%q) = %+v, want %+v", tt.importPath, actual, tt.expected)
		}
	}
}

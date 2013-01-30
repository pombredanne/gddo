// Copyright 2011 Gary Burd
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

package doc

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var defaultTags = map[string]string{"git": "master", "hg": "default"}

func bestTag(tags map[string]string, defaultTag string) (string, string, error) {
	if commit, ok := tags["go1"]; ok {
		return "go1", commit, nil
	}
	if commit, ok := tags[defaultTag]; ok {
		return defaultTag, commit, nil
	}
	return "", "", NotFoundError{"Tag or branch not found."}
}

// expand replaces {k} in template with match[k] or subs[atoi(k)] if k is not in match.
func expand(template string, match map[string]string, subs ...string) string {
	var p []byte
	var i int
	for {
		i = strings.Index(template, "{")
		if i < 0 {
			break
		}
		p = append(p, template[:i]...)
		template = template[i+1:]
		i = strings.Index(template, "}")
		if s, ok := match[template[:i]]; ok {
			p = append(p, s...)
		} else {
			j, _ := strconv.Atoi(template[:i])
			p = append(p, subs[j]...)
		}
		template = template[i+1:]
	}
	p = append(p, template...)
	return string(p)
}

var readmePat = regexp.MustCompile(`^[Rr][Ee][Aa][Dd][Mm][Ee](?:$|\.)`)

// isDocFile returns true if a file with name n should be included in the
// documentation.
func isDocFile(n string) bool {
	if strings.HasSuffix(n, ".go") && n[0] != '_' && n[0] != '.' {
		return true
	}
	return readmePat.MatchString(n)
}

// fetchFiles fetches the source files specified by the rawURL field in parallel.
func fetchFiles(client *http.Client, files []*source, header http.Header) error {
	ch := make(chan error, len(files))
	for i := range files {
		go func(i int) {
			req, err := http.NewRequest("GET", files[i].rawURL, nil)
			if err != nil {
				ch <- err
				return
			}
			for k, vs := range header {
				req.Header[k] = vs
			}
			resp, err := client.Do(req)
			if err != nil {
				ch <- &RemoteError{req.URL.Host, err}
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				ch <- &RemoteError{req.URL.Host, fmt.Errorf("get %s -> %d", req.URL, resp.StatusCode)}
				return
			}
			files[i].data, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				ch <- &RemoteError{req.URL.Host, err}
				return
			}
			ch <- nil
		}(i)
	}
	for _ = range files {
		if err := <-ch; err != nil {
			return err
		}
	}
	return nil
}

// httpGet gets the specified resource. ErrNotFound is returned if the
// server responds with status 404.
func httpGet(client *http.Client, url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &RemoteError{req.URL.Host, err}
	}
	if resp.StatusCode == 200 {
		return resp.Body, nil
	}
	resp.Body.Close()
	if resp.StatusCode == 404 { // 403 can be rate limit error.  || resp.StatusCode == 403 {
		err = NotFoundError{"Resource not found: " + url}
	} else {
		err = &RemoteError{req.URL.Host, fmt.Errorf("get %s -> %d", url, resp.StatusCode)}
	}
	return nil, err
}

func httpGetJSON(client *http.Client, url string, v interface{}) error {
	rc, err := httpGet(client, url)
	if err != nil {
		return err
	}
	defer rc.Close()
	err = json.NewDecoder(rc).Decode(v)
	if _, ok := err.(*json.SyntaxError); ok {
		err = NotFoundError{"JSON syntax error at " + url}
	}
	return err
}

// httpGet gets the specified resource. ErrNotFound is returned if the
// server responds with status 404.
func httpGetBytes(client *http.Client, url string) ([]byte, error) {
	rc, err := httpGet(client, url)
	if err != nil {
		return nil, err
	}
	p, err := ioutil.ReadAll(rc)
	rc.Close()
	return p, err
}

// httpGetBytesNoneMatch conditionally gets the specified resource. If a 304 status
// is returned, then the function returns ErrNotModified. If a 404
// status is returned, then the function returns ErrNotFound.
func httpGetBytesNoneMatch(client *http.Client, url string, etag string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("If-None-Match", `"`+etag+`"`)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", &RemoteError{req.URL.Host, err}
	}
	defer resp.Body.Close()

	etag = resp.Header.Get("Etag")
	if len(etag) >= 2 && etag[0] == '"' && etag[len(etag)-1] == '"' {
		etag = etag[1 : len(etag)-1]
	} else {
		etag = ""
	}

	switch resp.StatusCode {
	case 200:
		p, err := ioutil.ReadAll(resp.Body)
		return p, etag, err
	case 404:
		return nil, "", NotFoundError{"Resource not found: " + url}
	case 304:
		return nil, "", ErrNotModified
	default:
		return nil, "", &RemoteError{req.URL.Host, fmt.Errorf("get %s -> %d", url, resp.StatusCode)}
	}
	panic("unreachable")
}

// httpGet gets the specified resource. ErrNotFound is returned if the
// server responds with status 404. ErrNotModified is returned if the
// hash of the resource equals savedEtag.
func httpGetBytesCompare(client *http.Client, url string, savedEtag string) ([]byte, string, error) {
	p, err := httpGetBytes(client, url)
	if err != nil {
		return nil, "", err
	}
	h := md5.New()
	h.Write(p)
	etag := hex.EncodeToString(h.Sum(nil))
	if savedEtag == etag {
		err = ErrNotModified
	}
	return p, etag, err
}

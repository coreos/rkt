// Copyright 2018 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package distribution

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/PuerkitoBio/purell"
	"github.com/hashicorp/errwrap"
)

const (
	distOCILayoutVersion      = 0
	TypeOCILayout        Type = "oci-layout"
)

var OCIArchiveExtensions = map[string]bool{
	".tar":  true,
	".gzip": true,
	".zip":  true,
}

type OCILayout struct {
	cimdURL      *url.URL
	transportURL *url.URL

	str string
}

func NewOCILayout(u *url.URL) (Distribution, error) {
	c, err := parseCIMD(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", u.String(), err)
	}
	if c.Type != TypeOCILayout {
		return nil, fmt.Errorf("illegal OCI-LAYOUT archive distribution type: %q", c.Type)
	}

	data, err := url.QueryUnescape(c.Data)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("error unescaping url %q", c.Data), err)
	}
	ocil, err := url.Parse(data)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("error parsing  url %q", c.Data), err)
	}
	purell.NormalizeURL(u, purell.FlagSortQuery)

	str := u.String()
	if path := ocil.String(); OCIArchiveExtensions[filepath.Ext(path)] {
		str = path
	}

	return &OCILayout{
		cimdURL:      u,
		transportURL: ocil,
		str:          str,
	}, nil
}
func NewOCILayoutFromTransport(u *url.URL) (Distribution, error) {
	urlStr := NewCIMDString(TypeOCILayout, distOCILayoutVersion, url.QueryEscape(u.String()))
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return NewOCILayout(u)
}

func (a *OCILayout) CIMD() *url.URL {
	u, err := url.Parse(a.cimdURL.String())
	if err != nil {
		panic(err)
	}
	return u
}

func (a *OCILayout) Equals(d Distribution) bool {
	o2, ok := d.(*OCILayout)
	if !ok {
		return false
	}
	return a.CIMD().String() == o2.CIMD().String()
}

func (a *OCILayout) TransportURL() *url.URL {
	tu, err := url.Parse(a.transportURL.String())
	if err != nil {
		panic(fmt.Errorf("invalid transport URL:%v", err))
	}
	return tu
}

func (a *OCILayout) String() string {
	return a.str
}

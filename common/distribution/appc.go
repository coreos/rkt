// Copyright 2016 The rkt Authors
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
	"strings"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
)

const DistAppcVersion = 0

// Appc defines a distribution using appc image discovery
// Its format is cimd:appc:v=0:name?label01=....&label02=....
// The distribution type is "appc"
// The labels values must be Query escaped
// Example appc:coreos.com/etcd?version=v3.0.3&os=linux&arch=amd64
type Appc struct {
	u *url.URL
}

func NewAppc(rawuri string) (*Appc, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", rawuri, err)
	}
	dp, err := ParseDist(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", rawuri, err)
	}
	if dp.DistType != DistTypeAppc {
		return nil, fmt.Errorf("wrong scheme: %q", u.Scheme)
	}

	appcStr := dp.DistString
	for n, v := range u.Query() {
		appcStr += fmt.Sprintf(",%s=%s", n, v[0])
	}
	app, err := discovery.NewAppFromString(appcStr)
	if err != nil {
		return nil, fmt.Errorf("wrong appc image string %q: %v", rawuri, err)
	}

	return NewAppcFromApp(app), nil
}

func NewAppcFromApp(app *discovery.App) *Appc {
	rawuri := distBase(DistTypeAppc, DistAppcVersion) + app.Name.String()

	labels := types.Labels{}
	for n, v := range app.Labels {
		labels = append(labels, types.Label{Name: n, Value: v})
	}

	if len(labels) > 0 {
		queries := make([]string, len(labels))
		rawuri += "?"
		for i, l := range labels {
			queries[i] = fmt.Sprintf("%s=%s", l.Name, url.QueryEscape(l.Value))
		}
		rawuri += strings.Join(queries, "&")
	}

	u, err := url.Parse(rawuri)
	if err != nil {
		panic(fmt.Errorf("cannot parse URI %q: %v", rawuri, err))
	}
	// sort the query for reproducible strings
	sortQuery(u)
	return &Appc{u: u}
}

func (a *Appc) Type() DistType {
	return DistTypeAppc
}

func (a *Appc) URI() *url.URL {
	// Create a copy of the URL
	u, err := url.Parse(a.u.String())
	if err != nil {
		panic(err)
	}
	return u

}

func (a *Appc) ComparableURIString() string {
	return a.u.String()
}

func (a *Appc) App() *discovery.App {
	dp, err := ParseDist(a.u)
	if err != nil {
		panic(fmt.Errorf("cannot parse URI: %q: %v", a.u, err))
	}
	labels := map[types.ACIdentifier]string{}
	for n, v := range a.u.Query() {
		name, err := types.NewACIdentifier(n)
		if err != nil {
			panic(fmt.Errorf("cannot parse label name %s: %v", n, err))
		}
		labels[*name] = v[0]
	}
	name := dp.DistString
	app, err := discovery.NewApp(name, labels)
	if err != nil {
		panic(fmt.Errorf("failed to convert Appc distribution %q to a discovery.App: %v", a.u.String(), err))
	}
	return app
}

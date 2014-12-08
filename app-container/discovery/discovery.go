package discovery

import (
	"fmt"
	"io"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/net/html"
	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/net/html/atom"
)

type acMeta struct {
	name   string
	prefix string
	uri    string
}

type Endpoints struct {
	Sig  []string
	ACI  []string
	Keys []string
}

func appendMeta(meta []acMeta, attrs []html.Attribute) []acMeta {
	m := acMeta{}

	for _, a := range attrs {
		if a.Namespace != "" {
			continue
		}

		switch a.Key {
		case "name":
			m.name = a.Val

		case "content":
			parts := strings.SplitN(strings.TrimSpace(a.Val), " ", 2)
			if len(parts) < 2 {
				break
			}
			m.prefix = parts[0]
			m.uri = strings.TrimSpace(parts[1])
		}
	}

	// TODO(eyakubovich): should prefix be optional?
	if !strings.HasPrefix(m.name, "ac-") || m.prefix == "" || m.uri == "" {
		return meta
	}

	return append(meta, m)
}

func extractACMeta(r io.Reader) []acMeta {
	var meta []acMeta

	z := html.NewTokenizer(r)

	for {
		switch z.Next() {
		case html.ErrorToken:
			return meta

		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			if tok.DataAtom == atom.Meta {
				meta = appendMeta(meta, tok.Attr)
			}
		}
	}
}

func renderTemplate(tpl string, kvs ...string) string {
	for i := 0; i < len(kvs); i += 2 {
		k := kvs[i]
		v := kvs[i+1]
		tpl = strings.Replace(tpl, k, v, -1)
	}
	return tpl
}

func DiscoverEndpoints(app App, insecure bool) (*Endpoints, error) {
	var meta []acMeta

	// TODO(philips): make a more intelligent algorithm checking for if we
	// got actual useful metadata and if the URL can be fetched.
	parts := strings.Split(app.Name.String(), "/")
	for i := 0; i < len(parts); i++ {
		end := len(parts) - i
		_, body, err := httpsOrHTTP(strings.Join(parts[:end], "/"), insecure)
		if err != nil {
			return nil, err
		}

		meta = extractACMeta(body)
		body.Close()
		if len(meta) > 0 {
			break
		}
	}

	if len(meta) == 0 {
		return nil, fmt.Errorf("no discovery tags found")
	}

	tplVars := []string{"{os}", app.Labels["os"], "{arch}", app.Labels["arch"],
		"{name}", app.Name.String(), "{version}", app.Labels["version"]}

	de := &Endpoints{}

	for _, m := range meta {
		if !strings.HasPrefix(app.Name.String(), m.prefix) {
			continue
		}

		switch m.name {
		case "ac-discovery":
			m.uri = renderTemplate(m.uri, tplVars...)
			de.Sig = append(de.Sig, renderTemplate(m.uri, "{ext}", "sig"))
			de.ACI = append(de.ACI, renderTemplate(m.uri, "{ext}", "aci"))

		case "ac-discovery-pubkeys":
			de.Keys = append(de.Keys, m.uri)
		}
	}

	return de, nil
}

package discovery

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/net/html"
	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/net/html/atom"
)

type acMeta struct {
	name   string
	prefix string
	uri    string
}

type ACIEndpoint struct {
	ACI string
	Sig string
}

type Endpoints struct {
	ACIEndpoints []ACIEndpoint
	Keys         []string
}

const (
	defaultVersion = "latest"
)

var (
	templateExpression = regexp.MustCompile(`{.*?}`)
)

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

func renderTemplate(tpl string, kvs ...string) (string, bool) {
	for i := 0; i < len(kvs); i += 2 {
		k := kvs[i]
		v := kvs[i+1]
		tpl = strings.Replace(tpl, k, v, -1)
	}
	return tpl, !templateExpression.MatchString(tpl)
}

func createTemplateVars(app App) []string {
	tplVars := []string{"{name}", app.Name.String()}
	// If a label is called "name", it will be ignored as it appears after
	// in the slice
	for n, v := range app.Labels {
		tplVars = append(tplVars, fmt.Sprintf("{%s}", n), v)
	}
	return tplVars
}

func doDiscover(app App, pre string, insecure bool) (*Endpoints, error) {
	if app.Labels["version"] == "" {
		app.Labels["version"] = defaultVersion
	}

	_, body, err := httpsOrHTTP(pre, insecure)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	meta := extractACMeta(body)

	tplVars := createTemplateVars(app)

	de := &Endpoints{}

	for _, m := range meta {
		if !strings.HasPrefix(app.Name.String(), m.prefix) {
			continue
		}

		switch m.name {
		case "ac-discovery":
			// Ignore not handled variables as {ext} isn't already rendered.
			uri, _ := renderTemplate(m.uri, tplVars...)
			sig, ok := renderTemplate(uri, "{ext}", "sig")
			if !ok {
				continue
			}
			aci, ok := renderTemplate(uri, "{ext}", "aci")
			if !ok {
				continue
			}
			de.ACIEndpoints = append(de.ACIEndpoints, ACIEndpoint{ACI: aci, Sig: sig})

		case "ac-discovery-pubkeys":
			de.Keys = append(de.Keys, m.uri)
		}
	}

	if len(de.ACIEndpoints) == 0 {
		return nil, fmt.Errorf("found no ACI meta tags")
	}

	return de, nil
}

// DiscoverEndpoints will make HTTPS requests to find the ac-discovery meta
// tags and optionally will use HTTP if insecure is set. Based on the app
// passed the discovery templates will be filled out and returned in eps.
func DiscoverEndpoints(app App, insecure bool) (eps *Endpoints, err error) {
	parts := strings.Split(string(app.Name), "/")
	for i := range parts {
		end := len(parts) - i
		pre := strings.Join(parts[:end], "/")
		eps, err = doDiscover(app, pre, insecure)
		if err == nil {
			break
		}
	}

	return
}

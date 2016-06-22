package fetchers

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Headers map[string][]string

type InsecureOpts struct {
	AllowHTTP      bool `json:"allow_http"`
	SkipTLSCheck   bool `json:"skip_tls_check"`
	SkipImageCheck bool `json:"skip_image_check"`
}

type Config struct {
	Version       int                `json:"version"`
	Scheme        string             `json:"scheme"`
	Name          string             `json:"name"`
	InsecureOpts  InsecureOpts       `json:"insecure"`
	Debug         bool               `json:"debug"`
	Headers       map[string]Headers `json:"headers"`
	OutputACIPath string             `json:"output_aci_path"`
	OutputASCPath string             `json:"output_asc_path"`
}

type Result struct {
	Latest    bool   `json:"latest"`
	ETag      string `json:"etag"`
	MaxAge    int    `json:"max_age"`
	UseCached bool   `json:"use_cached"`
}

func ConfigFromStdin() (*Config, error) {
	confblob, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(confblob, config)
	return config, err
}

func ResultToStdout(res *Result) error {
	resBlob, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(resBlob)
	return err
}

package rest

import (
	"bytes"
	"fmt"
	"io"
	"os"

    "github.com/zigdon/rsp/cfg"
	"net/http"
	"net/url"
)

const (
	base = "https://api.replicant.space/v1"
)

var (
	client http.Client
)

func log(tmpl string, args ...any) {
	if os.Getenv("DEBUG_API") != "" {
		fmt.Fprintf(os.Stderr, tmpl, args...)
	}
}

func Get(path string, args ...any) ([]byte, error) {
	cfg, err := cfg.ReadCfg()
	if err != nil {
		return nil, err
	}
	url, err := url.Parse(fmt.Sprintf(base+"/"+path, args...))
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(&http.Request{
		Method: "GET",
		URL: url,
		Header: map[string][]string{
			"Authorization": {"Bearer "+cfg.APIKey},
		},
	})
	log("GET %q -> %d\n", url, resp.StatusCode)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET failed with %d:\n%s", resp.StatusCode, body)
	}

	return body, err
}

func Post(path string, data []byte, args ...any) ([]byte, error) {
	cfg, err := cfg.ReadCfg()
	if err != nil {
		return nil, err
	}
	url, err := url.Parse(fmt.Sprintf(base+"/"+path, args...))
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(&http.Request{
		Method: "POST",
		URL: url,
		Header: map[string][]string{
			"Authorization": {"Bearer "+cfg.APIKey},
		    "Content-Type": {"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(data)),
	})
	log("POST %q -> %d\n", url, resp.StatusCode)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("POST failed with %d:\n%s", resp.StatusCode, body)
	}

	return body, err
}

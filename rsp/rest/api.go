package rest

import (
	"fmt"
	"io"

    "github.com/zigdon/rsp/cfg"
	"net/http"
	"net/url"
)

const (
	base = "https://api.replicant.space/v1"
)

var client http.Client

func Get(path string, args ...any) (string, error) {
	cfg, err := cfg.ReadCfg()
	if err != nil {
		return "", err
	}
	url, err := url.Parse(fmt.Sprintf(base+"/"+path, args...))
	if err != nil {
		return "", err
	}
	resp, err := client.Do(&http.Request{
		Method: "GET",
		URL: url,
		Header: map[string][]string{
			"Authorization": []string{"Bearer "+cfg.APIKey},
		},
	})
	fmt.Printf("GET %q -> %d\n", url, resp.StatusCode)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return "", fmt.Errorf("GET failed with %d:\n%s", resp.StatusCode, body)
	}
	return string(body), err
}

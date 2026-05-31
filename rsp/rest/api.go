package rest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"net/http"
	"net/url"

	"github.com/zigdon/rsp/cfg"
	"github.com/zigdon/rsp/errors"
)

const (
	base = "https://api.replicant.space/v1"
	logFile = "/tmp/rsp.log"
)

var (
	client http.Client
)

func log(tmpl string, args ...any) {
	ts := time.Now().Format(time.Stamp)
	line := fmt.Sprintf(ts+" "+tmpl+"\n", args...)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't open to %q: %v\n", logFile, err)
	} else {
		f.WriteString(line)
		f.Close()
	}
	if os.Getenv("DEBUG_API") != "" {
		fmt.Fprint(os.Stderr, line)
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
	log("POST %q -> %d:\n%s", url, resp.StatusCode, string(data))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, errors.PostError{
			Err: fmt.Errorf("POST failed with %d:\n%s", resp.StatusCode, body),
			Status: resp.StatusCode,
			Body: body,
		}
	}

	return body, err
}

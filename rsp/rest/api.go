package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"net/http"
	"net/url"

	"github.com/zigdon/rsp/cfg"
)

const (
	base    = "https://api.replicant.space/v1"
	logFile = "/tmp/rsp-api.log"
)

var (
	client         http.Client
	UnreadMessages int
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

func do(method, path string, data []byte, args ...any) ([]byte, error) {
	cfg, err := cfg.ReadCfg()
	if err != nil {
		return nil, err
	}
	url, err := url.Parse(fmt.Sprintf(base+"/"+path, args...))
	if err != nil {
		return nil, err
	}
	start := time.Now()
	resp, err := client.Do(&http.Request{
		Method: method,
		URL:    url,
		Header: map[string][]string{
			"Authorization": {"Bearer " + cfg.APIKey},
			"Content-Type":  {"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(data)),
	})
	end := time.Now()
	log("%s %q -> %d (%s):\n%s", method, url, resp.StatusCode, end.Sub(start),string(data))
	if err != nil {
		log("err: %v", err)
		return nil, err
	}
	if resp.StatusCode == 404 {
		panic("404")
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	log("->\n%s", string(body))

	if unread, ok := resp.Header["X-Replicant-Space-Unread-Count"]; ok {
		UnreadMessages, err = strconv.Atoi(unread[0])
		if err != nil {
			log("Can't parse unread message count %v: %v", unread, err)
		}
	} else {
		UnreadMessages = 0
	}

	type jsonErrs struct {
		Error  string
		Errors []struct {
			DeviceCode string `json:"device_code"`
			Error      string
		}
	}
	var r jsonErrs
	if err = json.Unmarshal(body, &r); err != nil {
		// Couldn't extract errors from the message.
		return body, nil
	}

	var errs []error
	if r.Error != "" {
		errs = []error{errors.New(r.Error)}
	}
	for _, e := range r.Errors {
		errs = append(errs, fmt.Errorf("%s: %s", e.DeviceCode, e.Error))
	}
	return body, errors.Join(errs...)
}

func Patch(path string, data []byte, args ...any) ([]byte, error) {
	return do("PATCH", path, data, args...)
}

func Post(path string, data []byte, args ...any) ([]byte, error) {
	return do("POST", path, data, args...)
}

func Get(path string, args ...any) ([]byte, error) {
	return do("GET", path, nil, args...)
}

// / Cache
type cacheEntry struct {
	ts  time.Time
	res []byte
}

var cachedCalls sync.Map

func cachePOST(key string, ttl time.Duration, path string, data []byte, args ...any) ([]byte, error) {
	if ttl == 0 {
		ttl = time.Minute
	}
	if key == "" {
		key = fmt.Sprintf("%s:%v:%v", path, args, string(data))
	}
	now := time.Now()
	c, ok := cachedCalls.Load(key)
	ent, _ := c.(cacheEntry)
	if ok && now.Sub(ent.ts) <= ttl {
		return ent.res, nil
	}
	res, err := Post(path, data, args...)
	if err != nil {
		return nil, err
	}
	cachedCalls.Store(key, cacheEntry{
		ts:  now,
		res: res,
	})
	return res, nil
}

func cacheGET(key string, ttl time.Duration, path string, args ...any) ([]byte, error) {
	if ttl == 0 {
		ttl = time.Minute
	}
	if key == "" {
		key = fmt.Sprintf("%s:%v", path, args)
	}
	now := time.Now()
	c, ok := cachedCalls.Load(key)
	ent, _ := c.(cacheEntry)
	if ok && now.Sub(ent.ts) <= ttl {
		return ent.res, nil
	}
	res, err := Get(path, args...)
	if err != nil {
		return nil, err
	}
	cachedCalls.Store(key, cacheEntry{
		ts:  now,
		res: res,
	})
	return res, nil
}

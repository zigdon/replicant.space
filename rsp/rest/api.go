package rest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
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
	debug          bool
)

func init() {
	if os.Getenv("DEBUG_API") != "" {
		debug = true
	}
}

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
	if debug {
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
	backoff := 50 * time.Millisecond
	var resp *http.Response
	for {
		start := time.Now()
		resp, err = client.Do(&http.Request{
			Method: method,
			URL:    url,
			Header: map[string][]string{
				"Authorization": {"Bearer " + cfg.APIKey},
				"Content-Type":  {"application/json"},
			},
			Body: io.NopCloser(bytes.NewReader(data)),
		})
		end := time.Now()
		if len(data) > 0 {
			log("%s %q -> %d (%s)\n%s", method, url, resp.StatusCode, end.Sub(start).Round(10*time.Millisecond), string(data))
		} else {
			log("%s %q -> %d (%s)", method, url, resp.StatusCode, end.Sub(start).Round(10*time.Millisecond))
		}
		if err != nil {
			log("err: %v", err)
			return nil, err
		}
		if resp.StatusCode == 404 {
			panic("404")
		}
		if resp.StatusCode == 429 {
			log("Too many requests, backing off for %s", backoff)
			for k, v := range resp.Header {
				if strings.HasPrefix(k, "X-Ratelimit-") {
					log("  %s: %v", k, v)
				}
				if k == "X-Ratelimit-Reset" {
					if ts, err := strconv.Atoi(v[0]); err != nil {
						log("  Can't parse reset: %v", err)
					} else {
						reset := time.Unix(int64(ts), 0)
						backoff = time.Until(reset)
						log("  reset: %s (%s)", reset, backoff)
					}
				}
			}
			time.Sleep(backoff)
			backoff = slices.Min([]time.Duration{backoff * 2, time.Minute})
			continue
		}
		break
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()

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

func ReadStream(handler func(ev map[string]string) error) error {
	cfg, err := cfg.ReadCfg()
	if err != nil {
		return err
	}
	streamURL := base + "/events/stream"
	req, err := http.NewRequest(http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("Failed create stream request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	log("Conecting to stream...")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to connect to stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	log("Connected! Listening for incoming events...")

	scanner := bufio.NewScanner(resp.Body)
	currentEvent := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()

		// End of event block
		if line == "" {
			if len(currentEvent) != 0 {
				// Process or dispatch the completed event
				handler(currentEvent)
				// Reset the event container
				currentEvent = make(map[string]string)
			}
			continue
		}

		// Handle keep-alive messages
		if strings.HasPrefix(line, ":") {
			continue
		}
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			return fmt.Errorf("Bad event line: %q", line)
		}
		if _, ok := currentEvent[k]; ok {
			// Append to existing values
			currentEvent[k] += "\n" + v
		} else {
			currentEvent[k] = v
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("Stream read error: %v", err)
		}
	}

	return fmt.Errorf("Stream ended?!")
}

// Cache
type cacheEntry struct {
	ts  time.Time
	val any
}

var cachedCalls sync.Map
var CacheTimes sync.Map

func cachePOST(key string, ttl time.Duration, path string, data []byte, args ...any) ([]byte, error) {
	if ttl == 0 {
		def, ok := CacheTimes.Load(key)
		if ok {
			ttl = def.(time.Duration)
		} else {
			CacheTimes.Store(key, time.Minute)
			ttl = time.Minute
		}
	}
	if key == "" {
		key = fmt.Sprintf("POST %s:%v:%v", path, args, string(data))
	}
	c, ok := cachedCalls.Load(key)
	ent, _ := c.(cacheEntry)
	if ok && time.Since(ent.ts) <= ttl {
		return ent.val.([]byte), nil
	}
	res, err := Post(path, data, args...)
	if err != nil {
		return nil, err
	}
	cachedCalls.Store(key, cacheEntry{
		ts:  time.Now(),
		val: res,
	})
	return res, nil
}

func cacheGET(key string, ttl time.Duration, path string, args ...any) ([]byte, error) {
	if ttl == 0 {
		def, ok := CacheTimes.Load(key)
		if ok {
			ttl = def.(time.Duration)
		} else {
			CacheTimes.Store(key, time.Minute)
			ttl = time.Minute
		}
	}
	if key == "" {
		key = fmt.Sprintf("GET %s:%v", path, args)
	}
	now := time.Now()
	c, ok := cachedCalls.Load(key)
	ent, _ := c.(cacheEntry)
	if ok && now.Sub(ent.ts) <= ttl {
		return ent.val.([]byte), nil
	}
	res, err := Get(path, args...)
	if err != nil {
		return nil, err
	}
	cachedCalls.Store(key, cacheEntry{
		ts:  now,
		val: res,
	})
	return res, nil
}

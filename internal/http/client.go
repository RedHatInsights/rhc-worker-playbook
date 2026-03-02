// Based on yggdrasil's http client
// https://github.com/RedHatInsights/yggdrasil/blob/main/internal/http/client.go
// https://github.com/RedHatInsights/yggdrasil/blob/main/internal/work/dispatcher.go

package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redhatinsights/rhc-worker-playbook/internal/config"
	"github.com/subpop/go-log"
)

// Client is a specialized HTTP client, configured with mutual TLS certificate
// authentication.
type Client struct {
	http.Client
	userAgent string

	// Retries is the number of times the client will attempt to resend failed
	// HTTP requests before giving up.
	Retries int
}

// NewHTTPClient creates a client with the given TLS configuration and
// user-agent string.
func NewHTTPClient(config *tls.Config, ua string) *Client {
	client := http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
	client.Transport.(*http.Transport).TLSClientConfig = config.Clone()

	return &Client{
		Client:    client,
		userAgent: ua,
		Retries:   0,
	}
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}
	req.Header.Add("User-Agent", c.userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	return c.Do(req)
}

func (c *Client) Post(url string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	for k, v := range headers {
		req.Header.Add(k, strings.TrimSpace(v))
	}
	req.Header.Add("User-Agent", c.userAgent)

	log.Debugf("sending HTTP request: %v %v", req.Method, req.URL)
	log.Tracef("request: %v", req)

	return c.Do(req)
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	var attempt int

	for {
		if attempt > c.Retries {
			return nil, fmt.Errorf("cannot do HTTP request: too many retries")
		}
		resp, err = c.Client.Do(req)
		if err != nil {
			if err.(*url.Error).Timeout() {
				attempt++
				continue
			}
			return nil, fmt.Errorf("cannot do HTTP request: %v", err)
		}

		switch resp.StatusCode {
		case http.StatusServiceUnavailable, http.StatusTooManyRequests:
			value := resp.Header.Get("Retry-After")
			if value != "" {
				var when time.Time
				var err error

				when, err = time.Parse(time.RFC1123, value)
				if err != nil {
					d, err := time.ParseDuration(value + "s")
					if err != nil {
						return nil, fmt.Errorf("cannot parse Retry-After header: %v", err)
					}
					when = time.Now().Add(d)
				}
				time.Sleep(time.Until(when))
				attempt++
				continue
			}
		}

		log.Debugf("received HTTP response: %v", resp)

		return resp, nil
	}
}

// GetPlaybook wraps Client.Get and parses the response to a bytestring
func (c *Client) GetPlaybook(addr string) (content []byte, err error) {
	response, err := c.Get(addr)
	if err != nil {
		return nil, fmt.Errorf("cannot get detached message content: %v", err)
	}
	content, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %v", err)
	}
	if err = response.Body.Close(); err != nil {
		return nil, fmt.Errorf("cannot close response body: %v", err)
	}
	return content, nil
}

// PostResults wraps Client.Post and parses the response to useful variables
func (c *Client) PostResults(
	addr string,
	headers map[string]string,
	body []byte,
) (responseCode int, responseMetadata map[string]string, responseData []byte, err error) {
	URL, err := url.Parse(addr)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("cannot parse addr as URL: %v", err)
	}
	if URL.Scheme == "" {
		return -1, nil, nil, fmt.Errorf("URL: '%v' has no scheme", addr)
	}
	if config.DefaultConfig.DataHost != "" {
		URL.Host = config.DefaultConfig.DataHost
	}
	resp, err := c.Post(URL.String(), headers, body)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("cannot perform HTTP request: %v", err)
	}
	responseData, err = io.ReadAll(resp.Body)
	if err != nil {
		return -1, nil, nil, fmt.Errorf("cannot read HTTP response body: %v", err)
	}
	err = resp.Body.Close()
	if err != nil {
		return -1, nil, nil, fmt.Errorf("cannot close HTTP response body: %v", err)
	}
	responseCode = resp.StatusCode
	responseMetadata = make(map[string]string)
	for header := range resp.Header {
		responseMetadata[header] = resp.Header.Get(header)
	}

	return responseCode, responseMetadata, responseData, nil
}

package vegadns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const contentType = "application/x-www-form-urlencoded"

type Option func(*Client) error

func WithBasicAuth(user, pass string) Option {
	return func(c *Client) error {
		c.user = user
		c.pass = pass

		return nil
	}
}

func WithOAuth(key, secret string) Option {
	return func(c *Client) error {
		c.apiKey = key
		c.apiSecret = secret

		return nil
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		if client == nil {
			c.httpClient = client
		}

		return nil
	}
}

type Client struct {
	// Basic Auth
	user string
	pass string

	// OAuth
	apiKey    string
	apiSecret string

	httpClient *http.Client
	baseURL    *url.URL
	token      Token
}

// NewClient create a new [Client].
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	bu, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL: %w", err)
	}

	c := &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    bu.JoinPath("1.0"),
	}

	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Client) do(req *http.Request, expectedStatusCode int, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectedStatusCode {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("bad answer from VegaDNS (code: %d, message: %s)", resp.StatusCode, string(body))
	}

	if result == nil {
		return nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, result)
	if err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}

	return nil
}

func (c *Client) newRequest(ctx context.Context, method string, endpoint *url.URL, params url.Values) (*http.Request, error) {
	var (
		err error
		req *http.Request
	)

	if method == http.MethodGet || method == http.MethodDelete {
		endpoint.RawQuery = params.Encode()

		req, err = http.NewRequestWithContext(ctx, method, endpoint.String(), nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, endpoint.String(), strings.NewReader(params.Encode()))
	}

	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	err = c.setAuth(ctx, req)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	return req, nil
}

func (c *Client) setAuth(ctx context.Context, req *http.Request) error {
	switch {
	// Basic Auth
	case c.user != "" && c.pass != "":
		req.SetBasicAuth(c.user, c.pass)

	// OAuth
	case c.apiKey != "" && c.apiSecret != "":
		if c.token.valid() != nil {
			token, err := c.getAuthToken(ctx)
			if err != nil {
				return err
			}

			c.token = token
		}

		req.Header.Set("Authorization", c.token.formatBearer())
	}

	return nil
}

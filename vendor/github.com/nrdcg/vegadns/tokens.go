package vegadns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Token struct to hold token information.
type Token struct {
	Token     string    `json:"access_token"`
	TokenType string    `json:"token_type"`
	ExpiresIn int       `json:"expires_in"`
	ExpiresAt time.Time `json:"-"`
}

func (t Token) valid() error {
	if t.ExpiresAt.IsZero() || time.Now().UTC().After(t.ExpiresAt) {
		return errors.New("token expired")
	}

	return nil
}

func (t Token) formatBearer() string {
	return "Bearer " + t.Token
}

func (c *Client) getAuthToken(ctx context.Context) (Token, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL.JoinPath("token").String(), strings.NewReader(data.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("get auth token: %w", err)
	}

	req.SetBasicAuth(c.apiKey, c.apiSecret)

	req.Header.Set("Content-Type", contentType)

	issueTime := time.Now().UTC()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("get auth token: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return Token{}, fmt.Errorf("get auth token: bad answer from VegaDNS (code: %d, message: %s)", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("get auth token: response: %w", err)
	}

	tok := Token{}

	if err := json.Unmarshal(body, &tok); err != nil {
		return Token{}, fmt.Errorf("get auth token: unmarshalling body: %w", err)
	}

	if tok.TokenType != "bearer" {
		return Token{}, fmt.Errorf("get auth token: don't support anything except bearer tokens (token type: %s)", c.token.TokenType)
	}

	tok.ExpiresAt = issueTime.Add(time.Duration(tok.ExpiresIn) * time.Second)

	return tok, nil
}

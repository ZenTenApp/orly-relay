package regfishapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client struct holds the API client configuration
// including the base URL and the API key for authentication.
type Client struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewClient creates a new instance of the Regfish API client.
func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL: "https://api.regfish.de",
		APIKey:  apiKey,
		Client:  &http.Client{},
	}
}

// Request helper for making HTTP requests.
func (c *Client) Request(method, endpoint string, body interface{}, headers map[string]string) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)

	// Marshal body if provided
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, nil
}

// Record represents a DNS record with common fields.
type Record struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Data       string  `json:"data"`
	TTL        int     `json:"ttl,omitempty"`
	Priority   *int    `json:"priority,omitempty"`
	Annotation *string `json:"annotation,omitempty"`
	Tag        *string `json:"tag,omitempty"`
	Flags      *int    `json:"flags,omitempty"`
}

// GetRecord retrieves details about a specific DNS record by RRID.
func (c *Client) GetRecord(rrid int) (Record, error) {
	endpoint := fmt.Sprintf("/dns/rr/%d", rrid)
	respBody, err := c.Request("GET", endpoint, nil, nil)
	if err != nil {
		return Record{}, err
	}

	var response struct {
		Response Record `json:"response"`
	}

	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return Record{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Response, nil
}

// CreateRecord creates a new DNS record.
func (c *Client) CreateRecord(record Record) (Record, error) {
	respBody, err := c.Request("POST", "/dns/rr", record, nil)
	if err != nil {
		return Record{}, err
	}

	var response struct {
		Response Record `json:"response"`
	}

	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return Record{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Response, nil
}

// UpdateRecord updates a DNS record by the records' name
func (c *Client) UpdateRecord(record Record) (Record, error) {
	endpoint := fmt.Sprintf("/dns/rr")
	respBody, err := c.Request("PATCH", endpoint, record, nil)
	if err != nil {
		return Record{}, err
	}

	var response struct {
		Response Record `json:"response"`
	}

	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return Record{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Response, nil
}

// UpdateRecordById updates a DNS record by RRID.
func (c *Client) UpdateRecordById(rrid int, record Record) (Record, error) {
	endpoint := fmt.Sprintf("/dns/rr/%d", rrid)
	respBody, err := c.Request("PATCH", endpoint, record, nil)
	if err != nil {
		return Record{}, err
	}

	var response struct {
		Response Record `json:"response"`
	}

	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return Record{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Response, nil
}

// DeleteRecord deletes a DNS record by RRID.
func (c *Client) DeleteRecord(rrid int) error {
	endpoint := fmt.Sprintf("/dns/rr/%d", rrid)
	_, err := c.Request("DELETE", endpoint, nil, nil)
	return err
}

// GetRecordsByDomain retrieves all DNS records for a given domain.
func (c *Client) GetRecordsByDomain(domain string) ([]Record, error) {
	endpoint := fmt.Sprintf("/dns/%s/rr", domain)
	respBody, err := c.Request("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Response []Record `json:"response"`
	}

	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Response, nil
}

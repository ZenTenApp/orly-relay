package vegadns

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// Record struct representing a Record object.
// https://github.com/shupp/VegaDNS-API/blob/master/records_format.json
type Record struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	RecordType string `json:"record_type"`
	TTL        int    `json:"ttl"`
	RecordID   int    `json:"record_id"`
	LocationID string `json:"location_id"`
	DomainID   int    `json:"domain_id"`
}

// RecordsResponse api response list of records.
type RecordsResponse struct {
	Status  string   `json:"status"`
	Total   int      `json:"total_records"`
	Domain  Domain   `json:"domain"`
	Records []Record `json:"records"`
}

// GetRecordID helper to get the id of a record.
func (c *Client) GetRecordID(ctx context.Context, domainID int, name, recordType string) (int, error) {
	records, err := c.GetRecords(ctx, domainID)
	if err != nil {
		return -1, err
	}

	for _, r := range records {
		if r.Name == name && r.RecordType == recordType {
			return r.RecordID, nil
		}
	}

	return -1, errors.New("record not found")
}

// GetRecords retrieves all DNS records associated with the specified domain ID.
// https://generator.swagger.io/?url=https://raw.githubusercontent.com/shupp/VegaDNS-API/refs/heads/master/swagger/vegadns.swagger.json#/Records/get_records
func (c *Client) GetRecords(ctx context.Context, domainID int) ([]Record, error) {
	params := make(url.Values)
	params.Set("domain_id", strconv.Itoa(domainID))

	req, err := c.newRequest(ctx, http.MethodGet, c.baseURL.JoinPath("records"), params)
	if err != nil {
		return nil, err
	}

	answer := RecordsResponse{}

	err = c.do(req, http.StatusOK, &answer)
	if err != nil {
		return nil, err
	}

	return answer.Records, nil
}

// CreateTXTRecord creates a TXT record.
// https://generator.swagger.io/?url=https://raw.githubusercontent.com/shupp/VegaDNS-API/refs/heads/master/swagger/vegadns.swagger.json#/Records/post_records
func (c *Client) CreateTXTRecord(ctx context.Context, domainID int, name, value string, ttl int) error {
	params := make(url.Values)
	params.Set("record_type", "TXT")
	params.Set("ttl", strconv.Itoa(ttl))
	params.Set("domain_id", strconv.Itoa(domainID))
	params.Set("name", name)
	params.Set("value", value)

	req, err := c.newRequest(ctx, http.MethodPost, c.baseURL.JoinPath("records"), params)
	if err != nil {
		return err
	}

	return c.do(req, http.StatusCreated, nil)
}

// DeleteRecord deletes a record.
// https://generator.swagger.io/?url=https://raw.githubusercontent.com/shupp/VegaDNS-API/refs/heads/master/swagger/vegadns.swagger.json#/Records/delete_records__record_id_
func (c *Client) DeleteRecord(ctx context.Context, recordID int) error {
	req, err := c.newRequest(ctx, http.MethodDelete, c.baseURL.JoinPath("records", strconv.Itoa(recordID)), nil)
	if err != nil {
		return err
	}

	return c.do(req, http.StatusOK, nil)
}

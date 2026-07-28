package updateip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultYandexAPIURL = "https://cloud-api.yandex.net"
	maxAPIResponseSize  = 4 << 20
	recordsPageSize     = 9999
)

// YandexDNSClient implements DNSProvider through the Yandex 360 API.
type YandexDNSClient struct {
	BaseURL      string
	Organization string
	Domain       string
	Token        string
	Client       HTTPDoer
}

// Records returns every DNS record in the configured domain.
func (y YandexDNSClient) Records(ctx context.Context) ([]DNSRecord, error) {
	var response struct {
		Records []DNSRecord `json:"records"`
	}
	endpoint := fmt.Sprintf("%s?perPage=%d", y.recordsURL(), recordsPageSize)
	if err := y.doRequest(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}
	if response.Records == nil {
		return nil, errors.New("yandex response does not contain records")
	}
	return response.Records, nil
}

// Add creates a DNS record.
func (y YandexDNSClient) Add(ctx context.Context, value DNSRecordValue) error {
	return y.doRequest(ctx, http.MethodPost, y.recordsURL(), value, nil)
}

// Update replaces an existing DNS record.
func (y YandexDNSClient) Update(ctx context.Context, recordID int64, value DNSRecordValue) error {
	endpoint := fmt.Sprintf("%s/%d", y.recordsURL(), recordID)
	return y.doRequest(ctx, http.MethodPost, endpoint, value, nil)
}

func (y YandexDNSClient) recordsURL() string {
	return fmt.Sprintf(
		"%s/directory/v1/org/%s/domains/%s/dns",
		strings.TrimRight(y.BaseURL, "/"),
		url.PathEscape(y.Organization),
		url.PathEscape(y.Domain),
	)
}

// doRequest centralizes JSON transport so public methods expose only DNS operations.
func (y YandexDNSClient) doRequest(ctx context.Context, method, endpoint string, requestBody, responseBody any) error {
	request, err := y.newRequest(ctx, method, endpoint, requestBody)
	if err != nil {
		return err
	}
	response, err := y.Client.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	data, err := readYandexResponse(response)
	if err != nil {
		return err
	}
	if responseBody == nil {
		return nil
	}
	if err := json.Unmarshal(data, responseBody); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (y YandexDNSClient) newRequest(ctx context.Context, method, endpoint string, requestBody any) (*http.Request, error) {
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "OAuth "+y.Token)
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

// readYandexResponse bounds memory use and accepts every successful 2xx response.
func readYandexResponse(response *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(response.Body, maxAPIResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxAPIResponseSize {
		return nil, errors.New("yandex response is too large")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return nil, fmt.Errorf("yandex API returned HTTP %d: %s", response.StatusCode, message)
	}
	return data, nil
}

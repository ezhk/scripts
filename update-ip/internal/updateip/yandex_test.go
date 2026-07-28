package updateip

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func writeResponse(t *testing.T, writer io.Writer, body string) {
	t.Helper()
	if _, err := io.WriteString(writer, body); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func newTestYandexDNSClient(server *httptest.Server) YandexDNSClient {
	return YandexDNSClient{
		BaseURL: server.URL, Organization: "123456", Domain: "example.com", Token: "secret", Client: server.Client(),
	}
}

func TestDefaultYandexDNSRecordsURL(t *testing.T) {
	client := YandexDNSClient{
		BaseURL:      defaultYandexAPIURL,
		Organization: "123456",
		Domain:       "example.com",
	}

	const want = "https://api360.yandex.net/directory/v1/org/123456/domains/example.com/dns"
	if got := client.recordsURL(); got != want {
		t.Fatalf("recordsURL() = %q, want %q", got, want)
	}
}

func TestYandexDNSRequests(t *testing.T) {
	const recordsPath = "/directory/v1/org/123456/domains/example.com/dns"
	var writes []struct {
		path  string
		value DNSRecordValue
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "OAuth secret" {
			t.Errorf("Authorization = %q", got)
		}
		if request.URL.Path == recordsPath && request.Method == http.MethodGet {
			if got := request.URL.Query().Get("perPage"); got != "9999" {
				t.Errorf("perPage = %q", got)
			}
			writer.Header().Set("Content-Type", "application/json")
			writeResponse(t, writer, `{"records":[{"recordId":42,"type":"A","name":"home","address":"8.8.8.8"}]}`)
			return
		}

		if request.Method != http.MethodPost || !strings.HasPrefix(request.URL.Path, recordsPath) {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			http.Error(writer, "unexpected request", http.StatusBadRequest)
			return
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var value DNSRecordValue
		if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writes = append(writes, struct {
			path  string
			value DNSRecordValue
		}{path: request.URL.Path, value: value})
		writer.WriteHeader(http.StatusCreated)
		writeResponse(t, writer, `{}`)
	}))
	defer server.Close()

	client := newTestYandexDNSClient(server)

	records, err := client.Records(context.Background())
	if err != nil {
		t.Fatalf("Records() error = %v", err)
	}
	if len(records) != 1 || records[0].ID != 42 || records[0].Address != "8.8.8.8" {
		t.Fatalf("Records() = %+v", records)
	}

	value := DNSRecordValue{Type: recordTypeA, Name: "home", Address: "1.1.1.1", TTL: 300}
	if err := client.Add(context.Background(), value); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := client.Update(context.Background(), 42, value); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if len(writes) != 2 {
		t.Fatalf("writes = %d, want 2", len(writes))
	}
	if writes[0].path != recordsPath || writes[1].path != recordsPath+"/42" {
		t.Fatalf("write paths = %q, %q", writes[0].path, writes[1].path)
	}
	if writes[0].value != value || writes[1].value != value {
		t.Fatalf("write values = %+v", writes)
	}
}

func TestYandexDNSReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"denied"}`, http.StatusForbidden)
	}))
	defer server.Close()

	client := newTestYandexDNSClient(server)

	_, err := client.Records(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("Records() error = %v", err)
	}
}

func TestYandexDNSRequiresRecordsField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeResponse(t, writer, `{}`)
	}))
	defer server.Close()

	client := newTestYandexDNSClient(server)
	if _, err := client.Records(context.Background()); err == nil {
		t.Fatal("Records() error = nil")
	}
}

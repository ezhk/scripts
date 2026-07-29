package updateip

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type responseSpec struct {
	status int
	body   string
}

type stubHTTPClient struct {
	responses map[string]responseSpec
	requests  []*http.Request
}

func (c *stubHTTPClient) Do(request *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, request)
	spec := c.responses[request.URL.String()]
	return &http.Response{
		StatusCode: spec.status,
		Status:     http.StatusText(spec.status),
		Body:       io.NopCloser(strings.NewReader(spec.body)),
		Header:     make(http.Header),
	}, nil
}

func TestAddressFamilyDialConfig(t *testing.T) {
	tests := []struct {
		family  AddressFamily
		network string
		address string
	}{
		{family: IPv4Family, network: "tcp4", address: "0.0.0.0"},
		{family: IPv6Family, network: "tcp6", address: "::"},
	}

	for _, test := range tests {
		network, localAddress := test.family.dialConfig()
		if network != test.network || localAddress.IP.String() != test.address {
			t.Errorf("%s dial config = %q/%s, want %q/%s",
				test.family, network, localAddress.IP, test.network, test.address,
			)
		}
	}
}

func TestHTTPSAddressDetectorBypassesEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.example:8080")
	detector := newHTTPSAddressDetector(IPv4Family, "https://address.example/ip")
	client, ok := detector.Client.(*http.Client)
	if !ok {
		t.Fatalf("Client type = %T, want *http.Client", detector.Client)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("Transport.Proxy is configured, want direct discovery requests")
	}
}

func TestHTTPSAddressDetectorRejectsRedirects(t *testing.T) {
	detector := newHTTPSAddressDetector(IPv4Family, "https://address.example/ip")
	client, ok := detector.Client.(*http.Client)
	if !ok {
		t.Fatalf("Client type = %T, want *http.Client", detector.Client)
	}
	request, err := http.NewRequest(http.MethodGet, "http://address.example/ip", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if err := client.CheckRedirect(request, nil); err == nil {
		t.Fatal("CheckRedirect() error = nil")
	}
}

func TestHTTPSAddressDetectorUsesFallback(t *testing.T) {
	client := &stubHTTPClient{responses: map[string]responseSpec{
		"https://first.example/ip":  {status: http.StatusServiceUnavailable},
		"https://second.example/ip": {status: http.StatusOK, body: "8.8.8.8\n"},
	}}
	detector := HTTPSAddressDetector{
		Client:      client,
		Family:      IPv4Family,
		ServiceURLs: []string{"https://first.example/ip", "https://second.example/ip"},
	}

	address, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if want := netip.MustParseAddr("8.8.8.8"); address != want {
		t.Fatalf("Detect() = %s, want %s", address, want)
	}
	if len(client.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(client.requests))
	}
	if got := client.requests[1].Header.Get("User-Agent"); got != "update-ip/1" {
		t.Errorf("User-Agent = %q", got)
	}
}

func TestHTTPSAddressDetectorAcceptsPublicIPv6(t *testing.T) {
	client := &stubHTTPClient{responses: map[string]responseSpec{
		"https://v6.example/ip": {status: http.StatusOK, body: "2606:4700:4700::1111"},
	}}
	detector := HTTPSAddressDetector{Client: client, Family: IPv6Family, ServiceURLs: []string{"https://v6.example/ip"}}

	address, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if want := netip.MustParseAddr("2606:4700:4700::1111"); address != want {
		t.Fatalf("Detect() = %s, want %s", address, want)
	}
}

func TestHTTPSAddressDetectorRejectsUnsafeResponses(t *testing.T) {
	tests := map[string]struct {
		family AddressFamily
		body   string
	}{
		"current network":    {family: IPv4Family, body: "0.0.0.1"},
		"private address":    {family: IPv4Family, body: "192.168.1.10"},
		"shared address":     {family: IPv4Family, body: "100.64.0.1"},
		"documentation IPv4": {family: IPv4Family, body: "192.0.2.1"},
		"benchmark address":  {family: IPv4Family, body: "198.18.0.1"},
		"documentation IPv6": {family: IPv6Family, body: "2001:db8::1"},
		"reserved IPv6":      {family: IPv6Family, body: "5f00::1"},
		"wrong family":       {family: IPv4Family, body: "2606:4700:4700::1111"},
		"invalid response":   {family: IPv4Family, body: "not an address"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := &stubHTTPClient{responses: map[string]responseSpec{
				"https://address.example/ip": {status: http.StatusOK, body: test.body},
			}}
			detector := HTTPSAddressDetector{
				Client: client, Family: test.family, ServiceURLs: []string{"https://address.example/ip"},
			}
			if _, err := detector.Detect(context.Background()); err == nil {
				t.Fatal("Detect() error = nil")
			}
		})
	}
}

func TestHTTPSAddressDetectorRequiresHTTPS(t *testing.T) {
	client := &stubHTTPClient{}
	detector := HTTPSAddressDetector{Client: client, Family: IPv4Family, ServiceURLs: []string{"http://address.example/ip"}}

	if _, err := detector.Detect(context.Background()); err == nil {
		t.Fatal("Detect() error = nil")
	}
	if len(client.requests) != 0 {
		t.Fatalf("HTTP requests = %d, want 0", len(client.requests))
	}
}

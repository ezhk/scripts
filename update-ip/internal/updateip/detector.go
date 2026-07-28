package updateip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

const (
	maxAddressResponseSize  = 64
	addressServiceUserAgent = "update-ip/1"
	detectionTimeout        = 10 * time.Second
)

var (
	publicIPv6AddressPrefix = netip.MustParsePrefix("2000::/3")
	specialPurposePrefixes  = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.31.196.0/24"),
		netip.MustParsePrefix("192.52.193.0/24"),
		netip.MustParsePrefix("192.88.99.0/24"),
		netip.MustParsePrefix("192.175.48.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("2001::/23"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2002::/16"),
		netip.MustParsePrefix("3fff::/20"),
	}
)

// AddressFamily identifies an IP protocol family.
type AddressFamily uint8

const (
	// IPv4Family selects IPv4 addresses and sockets.
	IPv4Family AddressFamily = iota + 1
	// IPv6Family selects IPv6 addresses and sockets.
	IPv6Family
)

// String returns the conventional name of the address family.
func (family AddressFamily) String() string {
	switch family {
	case IPv4Family:
		return "IPv4"
	case IPv6Family:
		return "IPv6"
	default:
		return fmt.Sprintf("unknown(%d)", family)
	}
}

func (family AddressFamily) matches(address netip.Addr) bool {
	switch family {
	case IPv4Family:
		return address.Is4()
	case IPv6Family:
		return address.Is6()
	}
	return false
}

// dialConfig forces connections through the selected family by binding its unspecified local address.
func (family AddressFamily) dialConfig() (string, *net.TCPAddr) {
	switch family {
	case IPv4Family:
		return "tcp4", &net.TCPAddr{IP: net.IPv4zero}
	case IPv6Family:
		return "tcp6", &net.TCPAddr{IP: net.IPv6zero}
	default:
		return "", nil
	}
}

// HTTPDoer executes HTTP requests.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// AddressDetector discovers a public address visible outside the local network.
type AddressDetector interface {
	Detect(context.Context) (netip.Addr, error)
}

// HTTPSAddressDetector asks external services for the address seen by the
// internet. Its client is bound to the requested address family.
type HTTPSAddressDetector struct {
	Client      HTTPDoer
	Family      AddressFamily
	ServiceURLs []string
}

// newHTTPSAddressDetector builds a family-bound client so discovery services observe the intended route.
func newHTTPSAddressDetector(family AddressFamily, serviceURLs ...string) HTTPSAddressDetector {
	network, localAddress := family.dialConfig()
	dialer := &net.Dialer{LocalAddr: localAddress}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, _ string, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}

	return HTTPSAddressDetector{
		Client: &http.Client{
			Transport:     transport,
			Timeout:       detectionTimeout,
			CheckRedirect: rejectAddressServiceRedirect,
		},
		Family:      family,
		ServiceURLs: serviceURLs,
	}
}

func rejectAddressServiceRedirect(_ *http.Request, _ []*http.Request) error {
	return errors.New("address service redirects are not allowed")
}

// Detect returns the first valid public address reported by the configured services.
func (d HTTPSAddressDetector) Detect(ctx context.Context) (netip.Addr, error) {
	if d.Family != IPv4Family && d.Family != IPv6Family {
		return netip.Addr{}, fmt.Errorf("unsupported address family: %s", d.Family)
	}
	if d.Client == nil {
		return netip.Addr{}, errors.New("HTTP client is required")
	}

	var failures []error
	for _, serviceURL := range d.ServiceURLs {
		address, err := d.detectFromService(ctx, serviceURL)
		if err == nil {
			return address, nil
		}
		failures = append(failures, err)
	}
	if len(failures) == 0 {
		return netip.Addr{}, errors.New("no address detection services configured")
	}
	return netip.Addr{}, errors.Join(failures...)
}

func (d HTTPSAddressDetector) detectFromService(ctx context.Context, serviceURL string) (netip.Addr, error) {
	responseBody, err := d.fetchAddress(ctx, serviceURL)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%s: %w", serviceURL, err)
	}
	address, err := parsePublicAddress(responseBody, d.Family)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%s: %w", serviceURL, err)
	}
	return address, nil
}

// fetchAddress permits only HTTPS and bounds the response before parsing it as text.
func (d HTTPSAddressDetector) fetchAddress(ctx context.Context, serviceURL string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	if request.URL.Scheme != "https" {
		return "", errors.New("only HTTPS services are allowed")
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", addressServiceUserAgent)

	response, err := d.Client.Do(request)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxAddressResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxAddressResponseSize {
		return "", errors.New("response is too large")
	}
	return string(body), nil
}

// parsePublicAddress normalizes an address and rejects the wrong family or non-public ranges.
func parsePublicAddress(value string, family AddressFamily) (netip.Addr, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IP address: %w", err)
	}
	address = address.Unmap()
	if !family.matches(address) {
		return netip.Addr{}, fmt.Errorf("returned %s, want %s", address, family)
	}
	if !isPublicAddress(address) {
		return netip.Addr{}, fmt.Errorf("returned non-public address %s", address)
	}
	return address, nil
}

func isPublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() {
		return false
	}
	if address.Is6() && !publicIPv6AddressPrefix.Contains(address) {
		return false
	}
	for _, prefix := range specialPurposePrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

package updateip

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

type stubDetector struct {
	address netip.Addr
	err     error
}

func (d stubDetector) Detect(context.Context) (netip.Addr, error) {
	return d.address, d.err
}

type updateCall struct {
	id    int64
	value DNSRecordValue
}

type stubDNS struct {
	records      []DNSRecord
	recordsCalls int
	adds         []DNSRecordValue
	updates      []updateCall
}

func (d *stubDNS) Records(context.Context) ([]DNSRecord, error) {
	d.recordsCalls++
	return d.records, nil
}

func (d *stubDNS) Add(_ context.Context, value DNSRecordValue) error {
	d.adds = append(d.adds, value)
	return nil
}

func (d *stubDNS) Update(_ context.Context, id int64, value DNSRecordValue) error {
	d.updates = append(d.updates, updateCall{id: id, value: value})
	return nil
}

func testConfig() Config {
	return Config{
		Token:        "token",
		Organization: "123",
		Domain:       "example.com",
		Subdomain:    "home",
		TTL:          300,
	}
}

func newTestUpdater(dns *stubDNS, detectors ...AddressDetector) Updater {
	return Updater{Config: testConfig(), Detectors: detectors, DNS: dns}
}

func TestUpdaterChangesARecordToNATVisibleAddress(t *testing.T) {
	dns := &stubDNS{records: []DNSRecord{
		{ID: 42, Type: recordTypeA, Name: "home", Address: "203.0.113.10"},
	}}
	updater := newTestUpdater(dns,
		stubDetector{address: netip.MustParseAddr("8.8.8.8")},
		stubDetector{err: errors.New("IPv6 is unavailable")},
	)

	if err := updater.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(dns.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(dns.updates))
	}
	update := dns.updates[0]
	if update.id != 42 || update.value.Type != recordTypeA || update.value.Address != "8.8.8.8" {
		t.Fatalf("unexpected update: %+v", update)
	}
	if update.value.TTL != 300 || len(dns.adds) != 0 {
		t.Fatalf("update TTL/adds = %d/%d", update.value.TTL, len(dns.adds))
	}
}

func TestUpdaterPrefersIPv6OverIPv4(t *testing.T) {
	dns := &stubDNS{records: []DNSRecord{
		{ID: 1, Type: recordTypeA, Name: "home", Address: "9.9.9.9"},
		{ID: 2, Type: recordTypeAAAA, Name: "other", Address: "2606:4700:4700::1001"},
	}}
	updater := newTestUpdater(dns,
		stubDetector{address: netip.MustParseAddr("8.8.8.8")},
		stubDetector{address: netip.MustParseAddr("2606:4700:4700::1111")},
	)

	if err := updater.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(dns.updates) != 0 {
		t.Fatalf("updates = %d, want 0", len(dns.updates))
	}
	if len(dns.adds) != 1 {
		t.Fatalf("adds = %d, want 1", len(dns.adds))
	}
	if got := dns.adds[0]; got.Type != recordTypeAAAA || got.Address != "2606:4700:4700::1111" {
		t.Fatalf("unexpected add: %+v", got)
	}
}

func TestUpdaterComparesCanonicalIPv6Addresses(t *testing.T) {
	dns := &stubDNS{records: []DNSRecord{
		{ID: 2, Type: recordTypeAAAA, Name: "home", Address: "2606:4700:4700:0000:0000:0000:0000:1111"},
	}}
	updater := newTestUpdater(dns,
		stubDetector{address: netip.MustParseAddr("2606:4700:4700::1111")},
	)

	if err := updater.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(dns.updates) != 0 || len(dns.adds) != 0 {
		t.Fatalf("updates/adds = %d/%d, want 0/0", len(dns.updates), len(dns.adds))
	}
}

func TestUpdaterDoesNotCallDNSWhenDetectionFails(t *testing.T) {
	dns := &stubDNS{}
	updater := newTestUpdater(dns, stubDetector{err: errors.New("offline")})

	if err := updater.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil")
	}
	if dns.recordsCalls != 0 {
		t.Fatalf("Records() calls = %d, want 0", dns.recordsCalls)
	}
}

func TestUpdaterRejectsNonPublicDetectorAddress(t *testing.T) {
	dns := &stubDNS{}
	updater := newTestUpdater(dns,
		stubDetector{address: netip.MustParseAddr("192.168.1.10")},
	)

	if err := updater.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil")
	}
	if dns.recordsCalls != 0 {
		t.Fatalf("Records() calls = %d, want 0", dns.recordsCalls)
	}
}

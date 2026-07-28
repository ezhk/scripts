package updateip

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"go.uber.org/zap"
)

const (
	recordTypeA    = "A"
	recordTypeAAAA = "AAAA"
)

// DNSRecord describes an existing DNS record.
type DNSRecord struct {
	ID      int64  `json:"recordId"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// DNSRecordValue contains the writable fields of a DNS record.
type DNSRecordValue struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Address string `json:"address"`
	TTL     int    `json:"ttl"`
}

// DNSProvider lists and changes DNS records.
type DNSProvider interface {
	Records(context.Context) ([]DNSRecord, error)
	Add(context.Context, DNSRecordValue) error
	Update(context.Context, int64, DNSRecordValue) error
}

// Updater detects public addresses and synchronizes their DNS records.
type Updater struct {
	Config    Config
	Detectors []AddressDetector
	DNS       DNSProvider
	Logger    *zap.Logger
}

// Run performs one address-detection and DNS-update cycle for every available address family.
func (u Updater) Run(ctx context.Context) error {
	logger := u.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	addresses, err := u.detectAddresses(ctx, logger)
	if err != nil {
		return err
	}
	records, err := u.DNS.Records(ctx)
	if err != nil {
		return fmt.Errorf("list DNS records: %w", err)
	}

	for _, address := range addresses {
		if err := u.syncRecord(ctx, records, address, logger); err != nil {
			return err
		}
	}
	return nil
}

func (u Updater) syncRecord(ctx context.Context, records []DNSRecord, address netip.Addr, logger *zap.Logger) error {
	value := newDNSRecordValue(u.Config, address)
	record, found := findRecord(records, value.Type, value.Name)
	if found && addressesEqual(record.Address, address) {
		return nil
	}

	operation := "add"
	var changeErr error
	if found {
		operation = "update"
		changeErr = u.DNS.Update(ctx, record.ID, value)
	} else {
		changeErr = u.DNS.Add(ctx, value)
	}
	if changeErr != nil {
		return fmt.Errorf("%s %s record: %w", operation, value.Type, changeErr)
	}

	logger.Info("DNS record changed",
		zap.String("operation", operation),
		zap.String("type", value.Type),
		zap.String("name", value.Name),
		zap.String("address", value.Address),
	)
	return nil
}

func (u Updater) detectAddresses(ctx context.Context, logger *zap.Logger) ([]netip.Addr, error) {
	var addresses []netip.Addr
	var foundIPv4, foundIPv6 bool
	for _, detector := range u.Detectors {
		address, err := detector.Detect(ctx)
		if err != nil {
			logger.Warn("address detection failed", zap.Error(err))
			continue
		}
		address = address.Unmap()
		if !isPublicAddress(address) {
			logger.Warn("address detector returned a non-public address", zap.String("address", address.String()))
			continue
		}
		if address.Is4() {
			if foundIPv4 {
				continue
			}
			foundIPv4 = true
		} else {
			if foundIPv6 {
				continue
			}
			foundIPv6 = true
		}
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		return nil, errors.New("could not detect a public IPv4 or IPv6 address")
	}
	return addresses, nil
}

// newDNSRecordValue maps an address family to its corresponding DNS record type.
func newDNSRecordValue(config Config, address netip.Addr) DNSRecordValue {
	recordType := recordTypeA
	if address.Is6() {
		recordType = recordTypeAAAA
	}

	return DNSRecordValue{
		Type:    recordType,
		Name:    config.Subdomain,
		Address: address.String(),
		TTL:     config.TTL,
	}
}

func findRecord(records []DNSRecord, recordType, name string) (DNSRecord, bool) {
	for _, record := range records {
		if record.Type == recordType && record.Name == name {
			return record, true
		}
	}
	return DNSRecord{}, false
}

// addressesEqual compares parsed addresses so equivalent IPv6 spellings do not trigger updates.
func addressesEqual(value string, address netip.Addr) bool {
	current, err := netip.ParseAddr(value)
	return err == nil && current.Unmap() == address.Unmap()
}

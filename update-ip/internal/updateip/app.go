// Package updateip detects a host's public IP address and updates its Yandex DNS record.
package updateip

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	primaryAddressServiceURL   = "https://api64.ipify.org"
	secondaryAddressServiceURL = "https://icanhazip.com"
	yandexRequestTimeout       = 10 * time.Second
	updateTimeout              = 30 * time.Second
)

// Run wires the production dependencies and performs one bounded update attempt.
func Run(configPath string, logger *zap.Logger) error {
	config, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	detectors := []AddressDetector{
		newHTTPSAddressDetector(IPv6Family, primaryAddressServiceURL, secondaryAddressServiceURL),
		newHTTPSAddressDetector(IPv4Family, primaryAddressServiceURL, secondaryAddressServiceURL),
	}
	dns := YandexDNSClient{
		BaseURL:      defaultYandexAPIURL,
		Organization: config.Organization,
		Domain:       config.Domain,
		Token:        config.Token,
		Client:       &http.Client{Timeout: yandexRequestTimeout},
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	return Updater{
		Config:    config,
		Detectors: detectors,
		DNS:       dns,
		Logger:    logger,
	}.Run(ctx)
}

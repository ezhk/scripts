// Command update-ip detects this host's public IP address and updates its Yandex DNS record.
package main

import (
	"flag"
	"os"

	"github.com/ezhk/scripts/update-ip/internal/updateip"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", updateip.DefaultConfigPath, "path to the YAML configuration file")
	flag.Parse()

	logger := zap.Must(zap.NewProduction())
	defer func() {
		_ = logger.Sync()
	}()

	if err := updateip.Run(*configPath, logger); err != nil {
		logger.Error("update-ip failed", zap.Error(err))
		_ = logger.Sync()
		os.Exit(1)
	}
}

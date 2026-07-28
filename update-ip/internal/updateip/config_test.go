package updateip

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	if DefaultConfigPath != "/etc/update-ip.yml" {
		t.Fatalf("DefaultConfigPath = %q, want %q", DefaultConfigPath, "/etc/update-ip.yml")
	}
}

func TestParseConfig(t *testing.T) {
	input := `---
token: oauth-token
organization: 123456
domain: example.com
subdomain: home
ttl: 300
`

	config, err := parseConfig([]byte(input))
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	want := Config{
		Token: "oauth-token", Organization: "123456", Domain: "example.com", Subdomain: "home", TTL: 300,
	}
	if config != want {
		t.Errorf("parseConfig() = %+v, want %+v", config, want)
	}
}

func TestParseConfigRejectsInvalidValues(t *testing.T) {
	tests := map[string]struct {
		input   string
		wantErr string
	}{
		"missing token": {
			input:   "organization: 123\ndomain: example.com\nsubdomain: home\nttl: 300\n",
			wantErr: "token is required",
		},
		"missing organization": {
			input:   "token: token\ndomain: example.com\nsubdomain: home\nttl: 300\n",
			wantErr: "organization is required",
		},
		"missing domain": {
			input:   "token: token\norganization: 123\nsubdomain: home\nttl: 300\n",
			wantErr: "domain is required",
		},
		"missing subdomain": {
			input:   "token: token\norganization: 123\ndomain: example.com\nttl: 300\n",
			wantErr: "subdomain is required",
		},
		"zero organization": {
			input:   "token: token\norganization: 0\ndomain: example.com\nsubdomain: home\nttl: 300\n",
			wantErr: "organization must be a positive integer",
		},
		"zero ttl": {
			input:   "token: token\norganization: 123\ndomain: example.com\nsubdomain: home\nttl: 0\n",
			wantErr: "ttl must be a positive integer",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseConfig([]byte(test.input))
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("parseConfig() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestReadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "update_ip.yaml")
	config, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if config.TTL != 300 || config.Organization != "123456" {
		t.Fatalf("unexpected example config: %+v", config)
	}
}

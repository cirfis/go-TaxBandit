package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Configuration struct {
	CompanyProfile          CompanyDetails   `json:"company_profile"`
	PayrollParameters       PayrollDetails   `json:"payroll_parameters"`
	StatutoryTaxes          []TaxDetails     `json:"statutory_taxes"`
	InfrastructureEndpoints NetworkEndpoints `json:"infrastructure_endpoints"`
}

type CompanyDetails struct {
	LegalName string `json:"legal_name"`
	EIN       string `json:"ein"`
	Title     string `json:"title"`
}

type PayrollDetails struct {
	GrossWageCents           int64  `json:"gross_wage_cents"`
	NetPaycheckCents         int64  `json:"net_paycheck_cents"`
	Periodicity              string `json:"periodicity"`
	AnnualExpectedGrossCents int64  `json:"annual_expected_gross_cents"`
}

type TaxDetails struct {
	Name          string `json:"name"`
	LedgerAccount string `json:"ledger_account"`
	AmountCents   int64  `json:"amount_cents"`
}

type NetworkEndpoints struct {
	GoDBLedgerGRPC    string `json:"godbledger_grpc"`
	TaxBanditsAuthURL string `json:"taxbandits_auth_url"`
	TaxBanditsAPIURL  string `json:"taxbandits_api_url"`
}

func Load(explicitPath string) (*Configuration, error) {
	path, err := resolvePath(explicitPath)
	if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Configuration
	if err := json.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("malformed JSON syntax: %w", err)
	}
	return &cfg, nil
}

func resolvePath(explicitPath string) (string, error) {
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err == nil {
			return explicitPath, nil
		}
		return "", fmt.Errorf("explicit config not found at: %s", explicitPath)
	}

	home, _ := os.UserHomeDir()
	searchPaths := []string{
		"config.json",
		filepath.Join(home, ".config", "wh-engine", "config.json"),
		"/etc/wh-engine/config.json",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no config found in standard paths")
}

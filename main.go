/*
    go-TaxBandit - Unofficial Go implementation for the TaxBandits API
    Copyright (C) 2026 WASHING HANDS INC.

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as published
    by the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

        pb "github.com/darcys22/godbledger/proto/transaction"
	"google.golang.org/grpc"
)

const ConfigPath = "/home/cirfis/HOLD !/wh_config.json"

// --- CONFIGURATION SCHEMAS ---

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

// --- TAXBANDITS API SCHEMAS ---

type TaxBanditsAuth struct {
	StatusCode    int    `json:"StatusCode"`
	StatusMessage string `json:"StatusMessage"`
	AccessToken   string `json:"AccessToken"`
}

type FilingResponse struct {
	StatusCode   int         `json:"StatusCode"`
	StatusName   string      `json:"StatusName"`
	SubmissionId string      `json:"SubmissionId"`
	FormRecords  FormRecords `json:"FormRecords,omitempty"`
	Errors       []ApiError  `json:"Errors,omitempty"`
}

type FormRecords struct {
	ErrorRecords []ApiError `json:"ErrorRecords,omitempty"`
}

type ApiError struct {
	Id      string `json:"Id"`
	Name    string `json:"Name"`
	Message string `json:"Message"`
}

// --- SYSTEM LOGGING & FALLBACK ---

func emitLog(urgency, subject, body string) {
	// 1. Output to stdout for systemd journal tracking
	prefix := "[NORMAL]"
	if urgency == "CRITICAL" {
		prefix = "[CRITICAL ERROR]"
	}
	log.Printf("%s %s\n%s\n", prefix, subject, body)

	// 2. Attempt secure SMTP Delivery
	smtpHost := os.Getenv("WH_SMTP_HOST")
	smtpPort := os.Getenv("WH_SMTP_PORT")
	username := os.Getenv("WH_SMTP_USER")
	password := os.Getenv("WH_SMTP_PASS")
	toEmail := os.Getenv("WH_COMPLIANCE_EMAIL")

	if smtpHost == "" || username == "" || password == "" || toEmail == "" {
		log.Println("[WARN] SMTP configuration missing. Output restricted to local systemd journal.")
		return
	}

	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: [WH-COMPLIANCE] %s\r\nMIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n%s\r\n",
		username, toEmail, subject, body))

	auth := smtp.PlainAuth("", username, password, smtpHost)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, username, []string{toEmail}, msg)
	if err != nil {
		log.Printf("[ERROR] SMTP Transit Failure. Failed to send email: %v\n", err)
	}
}

// --- RESILIENT NETWORK LAYER ---

// doWithRetry wraps standard HTTP calls with an exponential backoff loop to survive temporary 503/network drops
func doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	var resp *http.Response
	var err error

	for i := 0; i < maxRetries; i++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil // Success or client-side error (400), break retry loop
		}
		
		log.Printf("[RETRY] Network drop or 500-level error. Retrying in %d seconds...\n", 2<<i)
		time.Sleep(time.Duration(2<<i) * time.Second)
	}
	return resp, fmt.Errorf("network request failed after %d attempts: %v", maxRetries, err)
}

// --- CRYPTOGRAPHIC ENGINE ---

func b64Raw(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func getTaxBanditsToken(cfg Configuration, clientID, clientSecret, userToken string) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]interface{}{
		"iss": clientID,
		"sub": clientID,
		"aud": userToken,
		"iat": time.Now().Unix(),
	})

	signingInput := fmt.Sprintf("%s.%s", b64Raw(header), b64Raw(payload))
	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(signingInput))
	signature := b64Raw(mac.Sum(nil))
	jwsToken := fmt.Sprintf("%s.%s", signingInput, signature)

	req, _ := http.NewRequest("GET", cfg.InfrastructureEndpoints.TaxBanditsAuthURL, nil)
	req.Header.Set("Authentication", jwsToken)

	resp, err := doWithRetry(req, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var authResp TaxBanditsAuth
	json.Unmarshal(body, &authResp)

	if authResp.StatusCode != 200 {
		return "", fmt.Errorf("gateway rejected handshake: %s", authResp.StatusMessage)
	}
	return authResp.AccessToken, nil
}

// --- CORE LOGIC TRACKS ---

func loadConfig() Configuration {
	file, err := os.ReadFile(ConfigPath)
	if err != nil {
		log.Fatalf("[-] Critical Error: Cannot locate config file: %v", err)
	}
	var cfg Configuration
	if err := json.Unmarshal(file, &cfg); err != nil {
		log.Fatalf("[-] Critical Error: Malformed JSON syntax in config: %v", err)
	}
	return cfg
}

// Track 1: Automated Local Ledger Logging
func executeMonthlyLedger(cfg Configuration) {
	currentMonth := time.Now().Format("January 2006")
	timestamp := time.Now().Format(time.RFC3339)

	conn, err := grpc.Dial(cfg.InfrastructureEndpoints.GoDBLedgerGRPC, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		emitLog("CRITICAL", "Monthly Ledger Failure: Database Unreachable", fmt.Sprintf("GoDBLedger gRPC daemon offline at %s.", cfg.InfrastructureEndpoints.GoDBLedgerGRPC))
		log.Fatalf("gRPC connection failure.")
	}
	defer conn.Close()
	client := pb.NewLedgerClient(conn)

	var splits []*pb.Split
	splits = append(splits, &pb.Split{AccountId: "Expenses:Payroll:GrossWages", Amount: cfg.PayrollParameters.GrossWageCents})
	splits = append(splits, &pb.Split{AccountId: "Assets:RelayChecking", Amount: -cfg.PayrollParameters.NetPaycheckCents})

	for _, tax := range cfg.StatutoryTaxes {
		splits = append(splits, &pb.Split{AccountId: tax.LedgerAccount, Amount: -tax.AmountCents})
	}

	transaction := &pb.TransactionRequest{
		Date:        time.Now().Format("2006-01-02"),
		Description: fmt.Sprintf("%s Payroll Invariant - %s", cfg.CompanyProfile.LegalName, currentMonth),
		Splits:      splits,
	}

	_, err = client.AddTransaction(context.Background(), transaction)
	if err != nil {
		emitLog("CRITICAL", "Monthly Ledger Rejected: Balance Discrepancy", "Transaction unbalanced. Verify integer math in wh_config.json.")
		log.Fatalf("Double-entry split rejection: %v", err)
	}

	report := fmt.Sprintf("Double-entry bookkeeping frame successfully committed for %s:\n\n[+] Expenses:Payroll:GrossWages: $%.2f\n[-] Assets:RelayChecking: $%.2f\n", 
		currentMonth, float64(cfg.PayrollParameters.GrossWageCents)/100, float64(cfg.PayrollParameters.NetPaycheckCents)/100)
	
	emitLog("NORMAL", fmt.Sprintf("Ledger Successfully Committed - %s", currentMonth), report)
}

// Track 2: Pre-Flight Math Validation & Tokenized API Transmission
func executeAnnualFiling(cfg Configuration) {
	year := time.Now().Format("2006")

	cID := os.Getenv("TB_CLIENT_ID")
	cSec := os.Getenv("TB_CLIENT_SECRET")
	uTok := os.Getenv("TB_USER_TOKEN")

	if cID == "" || cSec == "" || uTok == "" {
		emitLog("CRITICAL", "Transmission Blocked: Missing API Credentials", "Infrastructure environment tokens missing from execution context.")
		os.Exit(1)
	}

	// 1. SANITY CHECK: Enforce the $3,600.00 Mathematical Invariant
	conn, err := grpc.Dial(cfg.InfrastructureEndpoints.GoDBLedgerGRPC, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		emitLog("CRITICAL", "Pre-Flight Blocked: Database Offline", "Could not audit GoDBLedger before external transmission.")
		os.Exit(1)
	}
	defer conn.Close()
	client := pb.NewLedgerClient(conn)

	reportResp, err := client.GetAccountBalance(context.Background(), &pb.ReportRequest{AccountId: "Expenses:Payroll:GrossWages"})
	if err != nil || reportResp.Balance != cfg.PayrollParameters.AnnualExpectedGrossCents {
		emitLog("CRITICAL", "Transmission Intercepted: Math Discrepancy", fmt.Sprintf("Expected %d cents. Found %d cents. Refusing to file erratic data.", cfg.PayrollParameters.AnnualExpectedGrossCents, reportResp.Balance))
		os.Exit(1)
	}

	// 2. SECURE HANDSHAKE
	token, err := getTaxBanditsToken(cfg, cID, cSec, uTok)
	if err != nil {
		emitLog("CRITICAL", "Gateway Error: Token Rotation Failed", fmt.Sprintf("TaxBandits rejected cryptographic parameters: %v", err))
		os.Exit(1)
	}

	// 3. COMPILE PAYLOAD
	apiPayload := map[string]interface{}{
		"SubmissionManifest": map[string]string{
			"TxnId":           fmt.Sprintf("WHI-944-%s-%d", year, time.Now().Unix()),
			"IsFederalFiling": "TRUE",
		},
		"ReturnData": map[string]interface{}{
			"EIN":               cfg.CompanyProfile.EIN,
			"BusinessName":      cfg.CompanyProfile.LegalName,
			"TaxYear":           year,
			"WagesAmt":          float64(cfg.PayrollParameters.AnnualExpectedGrossCents) / 100,
			"FICAWithheld":      275.40,
			"TotalTaxLiability": 550.80,
		},
	}

	jsonPayload, _ := json.Marshal(apiPayload)
	req, _ := http.NewRequest("POST", cfg.InfrastructureEndpoints.TaxBanditsAPIURL+"/Form944/Create", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(req, 3)
	if err != nil {
		emitLog("CRITICAL", "Network Exception: Clearinghouse Timeout", fmt.Sprintf("Lost network connection to API gateway: %v", err))
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var filingResp FilingResponse
	json.Unmarshal(body, &filingResp)

	// 4. DEEP ERROR PARSING
	if filingResp.StatusCode == 200 || filingResp.StatusCode == 201 {
		emitLog("NORMAL", fmt.Sprintf("IRS TRANSMISSION ACCEPTED - %s", year), fmt.Sprintf("Gateway Status: 200 OK\nSubmission ID: %s", filingResp.SubmissionId))
	} else {
		var errorStrings []string
		// Parse high-level schema errors
		for _, e := range filingResp.Errors {
			errorStrings = append(errorStrings, fmt.Sprintf("Property: %s | Message: %s", e.Name, e.Message))
		}
		// Parse deep business-rule validation errors (e.g. mismatched TINs)
		for _, e := range filingResp.FormRecords.ErrorRecords {
			errorStrings = append(errorStrings, fmt.Sprintf("Business Rule: %s | Message: %s", e.Name, e.Message))
		}
		
		report := fmt.Sprintf("The IRS or Gateway Validation Engine rejected the payload:\n\n%s", strings.Join(errorStrings, "\n"))
		emitLog("CRITICAL", fmt.Sprintf("IRS TRANSMISSION REJECTED - %s", year), report)
	}
}

func main() {
	monthFlag := flag.Bool("month", false, "Execute configuration-driven monthly allocation")
	yearFlag := flag.Bool("year", false, "Execute pre-flight sanity checks and transmit annual forms")
	flag.Parse()

	cfg := loadConfig()

	if *monthFlag {
		executeMonthlyLedger(cfg)
	} else if *yearFlag {
		executeAnnualFiling(cfg)
	} else {
		fmt.Println("Washing Hands Core Engine\nUsage: wh-engine [--month | --year]")
	}
}

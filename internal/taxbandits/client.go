package taxbandits

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cirfis/go-TaxBandit/internal/config"
)

type Client struct {
	AuthURL      string
	APIURL       string
	ClientID     string
	ClientSecret string
	UserToken    string
	httpClient   *http.Client
}

func NewClient(authURL, apiURL, id, secret, user string) *Client {
	return &Client{
		AuthURL:      authURL,
		APIURL:       apiURL,
		ClientID:     id,
		ClientSecret: secret,
		UserToken:    user,
		httpClient:   &http.Client{Timeout: 20 * time.Second},
	}
}

func b64Raw(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func (c *Client) generateToken() (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]interface{}{
		"iss": c.ClientID,
		"sub": c.ClientID,
		"aud": c.UserToken,
		"iat": time.Now().Unix(),
	})

	signingInput := fmt.Sprintf("%s.%s", b64Raw(header), b64Raw(payload))
	mac := hmac.New(sha256.New, []byte(c.ClientSecret))
	mac.Write([]byte(signingInput))
	signature := b64Raw(mac.Sum(nil))
	jwsToken := fmt.Sprintf("%s.%s", signingInput, signature)

	req, _ := http.NewRequest("GET", c.AuthURL, nil)
	req.Header.Set("Authentication", jwsToken)

	resp, err := c.doWithRetry(req, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var authResp AuthResponse
	json.Unmarshal(body, &authResp)

	if authResp.StatusCode != 200 {
		return "", fmt.Errorf("gateway rejected handshake: %s", authResp.StatusMessage)
	}
	return authResp.AccessToken, nil
}

func (c *Client) SubmitForm944(cfg *config.Configuration) (*FilingResponse, error) {
	token, err := c.generateToken()
	if err != nil {
		return nil, fmt.Errorf("auth error: %w", err)
	}

	year := time.Now().Format("2006")
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
	req, _ := http.NewRequest("POST", c.APIURL+"/Form944/Create", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var filingResp FilingResponse
	json.Unmarshal(body, &filingResp)

	return &filingResp, nil
}

func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i < maxRetries; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		time.Sleep(time.Duration(2<<i) * time.Second)
	}
	return resp, fmt.Errorf("network failed after %d attempts: %v", maxRetries, err)
}

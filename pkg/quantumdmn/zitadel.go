package quantumdmn

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type zitadelKey struct {
	UserID string `json:"userId"`
	KeyID  string `json:"keyId"`
	Key    string `json:"key"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewZitadelTokenProviderWithProject creates a TokenProvider that authenticates using a Zitadel JSON Key file.
// It includes the project audience scope to get tokens that can be validated by the API.
//
// Parameters:
//   - keyPath: Path to the JSON key file downloaded from Zitadel Service Account.
//   - issuer: The Zitadel instance URL (e.g., https://auth.quantumdmn.com).
//   - projectID: The project ID to include in the audience (for granted projects).
func NewZitadelTokenProvider(keyPath string, issuer string, projectID string) (TokenProvider, error) {
	b, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var keyData zitadelKey
	if err := json.Unmarshal(b, &keyData); err != nil {
		return nil, fmt.Errorf("failed to parse key file: %w", err)
	}

	// parse RSA private key
	privateKey, err := parseRSAPrivateKey(keyData.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// build default scopes
	defaultScopes := []string{
		"openid",
		"profile",
		"urn:zitadel:iam:user:resourceowner",
		"urn:zitadel:iam:org:projects:roles",
	}

	// add project audience and roles scopes if projectID is provided
	if projectID != "" {
		defaultScopes = append(defaultScopes,
			"urn:zitadel:iam:org:project:id:"+projectID+":aud",
		)
	}

	// cache token
	var cachedToken string
	var tokenExpiry time.Time

	return func(ctx context.Context) (string, error) {
		// return cached token if still valid (with 1 minute buffer)
		if cachedToken != "" && time.Now().Before(tokenExpiry.Add(-time.Minute)) {
			return cachedToken, nil
		}

		// create JWT assertion
		now := time.Now()
		claims := jwt.MapClaims{
			"iss": keyData.UserID,
			"sub": keyData.UserID,
			"aud": issuer,
			"iat": now.Unix(),
			"exp": now.Add(time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = keyData.KeyID

		assertion, err := token.SignedString(privateKey)
		if err != nil {
			return "", fmt.Errorf("failed to sign JWT: %w", err)
		}

		// exchange JWT for access token
		tokenURL := strings.TrimSuffix(issuer, "/") + "/oauth/v2/token"

		data := url.Values{}
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
		data.Set("scope", strings.Join(defaultScopes, " "))
		data.Set("assertion", assertion)

		req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return "", fmt.Errorf("failed to create token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("token request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errBody map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&errBody)
			return "", fmt.Errorf("token request failed with status %d: %v", resp.StatusCode, errBody)
		}

		var tokenResp tokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return "", fmt.Errorf("failed to decode token response: %w", err)
		}

		cachedToken = tokenResp.AccessToken
		fmt.Println("Token: ", cachedToken)
		tokenExpiry = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

		return cachedToken, nil
	}, nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// try PKCS#1 first (RSA PRIVATE KEY)
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// try PKCS#8 (PRIVATE KEY)
	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := keyInterface.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA private key")
	}

	return rsaKey, nil
}

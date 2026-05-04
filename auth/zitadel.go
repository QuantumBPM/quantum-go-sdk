package auth

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
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ZitadelTokenProvider authenticates against a Zitadel instance using a
// service-account JSON Key file (the JWT Profile grant). Acquired tokens are
// cached in-memory until shortly before expiry.
type ZitadelTokenProvider struct {
	keyData    zitadelKey
	privateKey *rsa.PrivateKey
	issuer     string
	scopes     []string
	httpClient *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

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

// NewZitadelTokenProvider builds a provider from a Zitadel service-account
// key file. issuer is the Zitadel instance base URL (e.g.
// https://auth.quantumbpm.com). projectID, when set, adds the project audience
// scope so issued tokens are accepted by the platform API.
func NewZitadelTokenProvider(keyPath, issuer, projectID string) (*ZitadelTokenProvider, error) {
	b, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("auth: read key file: %w", err)
	}

	var keyData zitadelKey
	if err := json.Unmarshal(b, &keyData); err != nil {
		return nil, fmt.Errorf("auth: parse key file: %w", err)
	}

	privateKey, err := parseRSAPrivateKey(keyData.Key)
	if err != nil {
		return nil, fmt.Errorf("auth: parse private key: %w", err)
	}

	scopes := []string{
		"openid",
		"profile",
		"urn:zitadel:iam:user:resourceowner",
		"urn:zitadel:iam:org:projects:roles",
	}
	if projectID != "" {
		scopes = append(scopes, "urn:zitadel:iam:org:project:id:"+projectID+":aud")
	}

	return &ZitadelTokenProvider{
		keyData:    keyData,
		privateKey: privateKey,
		issuer:     strings.TrimSuffix(issuer, "/"),
		scopes:     scopes,
		httpClient: http.DefaultClient,
	}, nil
}

// Token returns a cached bearer token if still valid, otherwise exchanges a
// freshly-signed JWT assertion for a new access token.
func (p *ZitadelTokenProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry.Add(-time.Minute)) {
		return p.cachedToken, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": p.keyData.UserID,
		"sub": p.keyData.UserID,
		"aud": p.issuer,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken.Header["kid"] = p.keyData.KeyID

	assertion, err := jwtToken.SignedString(p.privateKey)
	if err != nil {
		return "", fmt.Errorf("auth: sign JWT: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("scope", strings.Join(p.scopes, " "))
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, "POST", p.issuer+"/oauth/v2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("auth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth: token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return "", fmt.Errorf("auth: token request failed (%d): %v", resp.StatusCode, body)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("auth: decode token response: %w", err)
	}

	p.cachedToken = tokenResp.AccessToken
	p.tokenExpiry = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return p.cachedToken, nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("decode PEM block")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA")
	}
	return rsaKey, nil
}

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// refreshMargin is how long before expiry a cached token is replaced.
	refreshMargin = 60 * time.Second
	// defaultTokenLifetime applies when the token response has no expires_in.
	defaultTokenLifetime = 300 * time.Second
)

// ClientCredentials configures a ClientCredentialsTokenProvider.
type ClientCredentials struct {
	// TokenURL is the identity provider's OAuth 2.0 token endpoint. Required.
	TokenURL string
	// ClientID identifies the client at the identity provider. Required.
	ClientID string
	// ClientSecret authenticates the client. Required.
	ClientSecret string
	// Scopes are sent space-joined as the scope parameter. Empty sends none.
	Scopes []string
	// Audience is sent as the audience parameter when set. Some providers,
	// such as Auth0, require it.
	Audience string
	// HTTPClient sends the token requests. Defaults to http.DefaultClient.
	HTTPClient *http.Client
}

// ClientCredentialsTokenProvider fetches access tokens from any OIDC provider
// with the OAuth 2.0 client credentials grant. The token is cached and
// replaced once less than 60 seconds of its lifetime remain. It is safe for
// concurrent use.
type ClientCredentialsTokenProvider struct {
	cfg ClientCredentials

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

type tokenResponse struct {
	AccessToken string      `json:"access_token"`
	ExpiresIn   json.Number `json:"expires_in"`
}

// NewClientCredentialsTokenProvider builds a provider from cfg. A missing
// TokenURL, ClientID or ClientSecret is reported by Token.
func NewClientCredentialsTokenProvider(cfg ClientCredentials) *ClientCredentialsTokenProvider {
	return &ClientCredentialsTokenProvider{cfg: cfg}
}

// Token returns the cached access token while more than 60 seconds of its
// lifetime remain, otherwise it requests a new one from the token endpoint.
func (p *ClientCredentialsTokenProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cachedToken != "" && time.Until(p.tokenExpiry) > refreshMargin {
		return p.cachedToken, nil
	}
	if p.cfg.TokenURL == "" || p.cfg.ClientID == "" || p.cfg.ClientSecret == "" {
		return "", errors.New("auth: TokenURL, ClientID and ClientSecret are required")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	if len(p.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(p.cfg.Scopes, " "))
	}
	if p.cfg.Audience != "" {
		form.Set("audience", p.cfg.Audience)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("auth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpClient := p.cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	requestedAt := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("auth: read token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("auth: token request failed (%d): %s", resp.StatusCode, body)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("auth: decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", errors.New("auth: token response has no access_token")
	}

	lifetime := defaultTokenLifetime
	if tokenResp.ExpiresIn != "" {
		seconds, err := tokenResp.ExpiresIn.Float64()
		if err != nil {
			return "", fmt.Errorf("auth: parse expires_in: %w", err)
		}
		lifetime = time.Duration(seconds * float64(time.Second))
	}

	p.cachedToken = tokenResp.AccessToken
	p.tokenExpiry = requestedAt.Add(lifetime)
	return p.cachedToken, nil
}

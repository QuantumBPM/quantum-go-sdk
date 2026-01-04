package quantumdmn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/oauth2/jwt"
)

type zitadelKey struct {
	UserID string `json:"userId"`
	KeyID  string `json:"keyId"`
	Key    string `json:"key"`
}

// NewZitadelTokenProvider creates a TokenProvider that authenticates using a Zitadel JSON Key file.
// It uses standard OAuth2 JWT Profile authentication.
//
// Parameters:
//   - keyPath: Path to the JSON key file downloaded from Zitadel Service Account.
//   - issuer: The Zitadel instance URL (e.g., https://your-instance.zitadel.cloud).
//   - scopes: Optional OAuth2 scopes (default: openid, profile, urn:zitadel:iam:user:resourceowner).
func NewZitadelTokenProvider(keyPath string, issuer string, scopes ...string) (TokenProvider, error) {
	b, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var keyData zitadelKey
	if err := json.Unmarshal(b, &keyData); err != nil {
		return nil, fmt.Errorf("failed to parse key file: %w", err)
	}

	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "urn:zitadel:iam:user:resourceowner"}
	}

	// Configure JWT Flow
	conf := &jwt.Config{
		// For Zitadel Service Accounts, the JWT 'iss' and 'sub' should be the UserID.
		// golang.org/x/oauth2/jwt sets 'iss' = Email.
		Email:      keyData.UserID,
		Subject:    keyData.UserID,
		PrivateKey: []byte(keyData.Key),
		TokenURL:   issuer + "/oauth/v2/token",
		Scopes:     scopes,
	}

	return func(ctx context.Context) (string, error) {
		ts := conf.TokenSource(ctx)
		tok, err := ts.Token()
		if err != nil {
			return "", err
		}
		return tok.AccessToken, nil
	}, nil
}

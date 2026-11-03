// Package auth provides token providers and an authenticated HTTP client
// wrapper for the QuantumBPM SDK. ClientCredentialsTokenProvider fetches
// tokens from any OIDC provider and StaticTokenProvider supplies a fixed
// bearer token. Implement TokenProvider to plug in any other token source.
package auth

import (
	"context"
	"net/http"

	"github.com/QuantumBPM/quantum-go-sdk/v2/generated"
)

// TokenProvider returns a valid access token for the next request. The
// provider is invoked on every request and is responsible for caching.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// TokenProviderFunc adapts a plain function to the TokenProvider interface.
type TokenProviderFunc func(ctx context.Context) (string, error)

func (f TokenProviderFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

// NewClient returns a generated.ClientWithResponses that injects a Bearer
// token from the supplied TokenProvider into every request.
func NewClient(baseURL string, provider TokenProvider) (*generated.ClientWithResponses, error) {
	editor := func(ctx context.Context, req *http.Request) error {
		token, err := provider.Token(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
	return generated.NewClientWithResponses(baseURL, generated.WithRequestEditorFn(editor))
}

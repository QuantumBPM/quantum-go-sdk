// package quantumdmn provides a QuantumDMN client wrapper with authentication helpers.
package quantumdmn

import (
	"context"
	"net/http"
)

// TokenProvider is a function that returns a valid access token.
// Implement this to integrate with your authentication system (e.g., Zitadel JWT Profile).
type TokenProvider func(ctx context.Context) (string, error)

// NewAuthenticatedClient creates a new QuantumDMN API client with automatic token injection.
// The tokenProvider is called for each request to get a valid access token.
func NewAuthenticatedClient(baseURL string, tokenProvider TokenProvider) (*ClientWithResponses, error) {
	authEditor := func(ctx context.Context, req *http.Request) error {
		token, err := tokenProvider(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	return NewClientWithResponses(baseURL, WithRequestEditorFn(authEditor))
}

// NewClientWithToken creates a client with a static bearer token.
// Useful for testing or when you manage token lifecycle externally.
func NewClientWithToken(baseURL, token string) (*ClientWithResponses, error) {
	authEditor := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
	return NewClientWithResponses(baseURL, WithRequestEditorFn(authEditor))
}

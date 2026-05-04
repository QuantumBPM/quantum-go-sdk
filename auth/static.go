package auth

import "context"

// StaticTokenProvider returns the same bearer token on every call. Use it for
// Enterprise deployments that issue long-lived API keys, or in tests where a
// pre-acquired token is supplied directly.
type StaticTokenProvider struct {
	token string
}

// NewStaticTokenProvider wraps token in a TokenProvider.
func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: token}
}

func (p *StaticTokenProvider) Token(_ context.Context) (string, error) {
	return p.token, nil
}

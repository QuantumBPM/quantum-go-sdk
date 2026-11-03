package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// tokenStub serves token responses and records each request's form.
type tokenStub struct {
	server   *httptest.Server
	requests atomic.Int32

	mu    sync.Mutex
	forms []url.Values
}

// newTokenStub answers every request with status and the body built by
// respond, which receives the 1-based request number.
func newTokenStub(t *testing.T, status int, respond func(n int32) string) *tokenStub {
	t.Helper()
	s := &tokenStub{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := s.requests.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", ct)
		}
		s.mu.Lock()
		s.forms = append(s.forms, r.PostForm)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respond(n)))
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *tokenStub) firstForm() url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forms[0]
}

func (s *tokenStub) provider(scopes []string, audience string) *ClientCredentialsTokenProvider {
	return NewClientCredentialsTokenProvider(ClientCredentials{
		TokenURL:     s.server.URL,
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		Scopes:       scopes,
		Audience:     audience,
	})
}

func tokenJSON(expiresIn string) func(n int32) string {
	return func(n int32) string {
		return fmt.Sprintf(`{"access_token":"token-%d","token_type":"Bearer"%s}`, n, expiresIn)
	}
}

func mustToken(t *testing.T, p *ClientCredentialsTokenProvider) string {
	t.Helper()
	token, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	return token
}

func TestClientCredentials_SendsScopesAndAudience(t *testing.T) {
	stub := newTokenStub(t, http.StatusOK, tokenJSON(`,"expires_in":3600`))
	mustToken(t, stub.provider([]string{"openid", "bpm.api"}, "quantumbpm-api"))

	want := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"my-client"},
		"client_secret": {"my-secret"},
		"scope":         {"openid bpm.api"},
		"audience":      {"quantumbpm-api"},
	}
	if got := stub.firstForm(); got.Encode() != want.Encode() {
		t.Fatalf("form = %v, want %v", got, want)
	}
}

func TestClientCredentials_OmitsEmptyScopesAndAudience(t *testing.T) {
	stub := newTokenStub(t, http.StatusOK, tokenJSON(`,"expires_in":3600`))
	mustToken(t, stub.provider(nil, ""))

	want := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"my-client"},
		"client_secret": {"my-secret"},
	}
	if got := stub.firstForm(); got.Encode() != want.Encode() {
		t.Fatalf("form = %v, want %v", got, want)
	}
}

func TestClientCredentials_ReusesCachedToken(t *testing.T) {
	cases := map[string]string{
		"number":         `,"expires_in":3600`,
		"numeric string": `,"expires_in":"3600"`,
		"missing":        ``,
	}
	for name, expiresIn := range cases {
		t.Run(name, func(t *testing.T) {
			stub := newTokenStub(t, http.StatusOK, tokenJSON(expiresIn))
			p := stub.provider(nil, "")

			first, second := mustToken(t, p), mustToken(t, p)
			if first != "token-1" || second != "token-1" {
				t.Fatalf("tokens = %q, %q, want token-1 twice", first, second)
			}
			if n := stub.requests.Load(); n != 1 {
				t.Fatalf("requests = %d, want 1", n)
			}
		})
	}
}

func TestClientCredentials_RefreshesNearExpiry(t *testing.T) {
	// 30 seconds is inside the 60 second refresh margin.
	stub := newTokenStub(t, http.StatusOK, tokenJSON(`,"expires_in":30`))
	p := stub.provider(nil, "")

	first, second := mustToken(t, p), mustToken(t, p)
	if first != "token-1" || second != "token-2" {
		t.Fatalf("tokens = %q, %q, want token-1 then token-2", first, second)
	}
	if n := stub.requests.Load(); n != 2 {
		t.Fatalf("requests = %d, want 2", n)
	}
}

func TestClientCredentials_ConcurrentCallersShareOneRequest(t *testing.T) {
	stub := newTokenStub(t, http.StatusOK, tokenJSON(`,"expires_in":3600`))
	p := stub.provider(nil, "")

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Token(context.Background()); err != nil {
				t.Errorf("Token: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := stub.requests.Load(); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
}

func TestClientCredentials_ErrorOnUnauthorized(t *testing.T) {
	stub := newTokenStub(t, http.StatusUnauthorized, func(int32) string { return `{"error":"invalid_client"}` })

	_, err := stub.provider(nil, "").Token(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if msg := err.Error(); !strings.Contains(msg, "401") || !strings.Contains(msg, "invalid_client") {
		t.Fatalf("error %q lacks status or body", msg)
	}
}

func TestClientCredentials_RequiresCredentials(t *testing.T) {
	p := NewClientCredentialsTokenProvider(ClientCredentials{TokenURL: "http://127.0.0.1:1/token"})
	if _, err := p.Token(context.Background()); err == nil {
		t.Fatal("expected an error for missing ClientID and ClientSecret")
	}
}

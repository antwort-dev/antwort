package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AuthProvider supplies authentication headers for MCP server connections.
type AuthProvider interface {
	// GetHeaders returns the HTTP headers to include in MCP requests.
	GetHeaders(ctx context.Context) (map[string]string, error)
}

// StaticKeyAuth provides authentication via static headers configured
// at initialization time. Suitable for API key authentication.
type StaticKeyAuth struct {
	// Headers contains the static authentication headers.
	Headers map[string]string
}

// GetHeaders returns the configured static headers.
func (a *StaticKeyAuth) GetHeaders(_ context.Context) (map[string]string, error) {
	return a.Headers, nil
}

// OAuthClientCredentialsAuth obtains access tokens via OAuth 2.0 client_credentials grant.
// Tokens are cached and proactively refreshed when 80% of the token lifetime has elapsed.
// If a proactive refresh fails but the cached token is still valid, the cached token is used.
type OAuthClientCredentialsAuth struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
	refreshAt   time.Time
	httpClient  *http.Client
	nowFunc     func() time.Time // for testing; defaults to time.Now
}

// tokenResponse represents the JSON response from an OAuth 2.0 token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewOAuthClientCredentials creates an OAuthClientCredentialsAuth provider.
func NewOAuthClientCredentials(tokenURL, clientID, clientSecret string, scopes []string) *OAuthClientCredentialsAuth {
	return &OAuthClientCredentialsAuth{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		nowFunc:      time.Now,
	}
}

// GetHeaders returns an Authorization header with a Bearer token.
// It caches tokens and proactively refreshes them when 80% of the lifetime has elapsed.
func (a *OAuthClientCredentialsAuth) GetHeaders(ctx context.Context) (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.nowFunc()

	// If we have a cached token and haven't reached the proactive refresh point, use it.
	if a.cachedToken != "" && now.Before(a.refreshAt) {
		return map[string]string{"Authorization": "Bearer " + a.cachedToken}, nil
	}

	// Attempt to acquire a new token.
	token, expiresIn, err := a.fetchToken(ctx)
	if err != nil {
		// If the cached token is still valid (not expired), use it despite refresh failure.
		if a.cachedToken != "" && now.Before(a.tokenExpiry) {
			return map[string]string{"Authorization": "Bearer " + a.cachedToken}, nil
		}
		return nil, fmt.Errorf("acquiring OAuth token: %w", err)
	}

	// Cache the new token.
	a.cachedToken = token
	a.tokenExpiry = now.Add(time.Duration(expiresIn) * time.Second)
	// Proactively refresh at 80% of the token's lifetime.
	refreshDuration := time.Duration(float64(expiresIn)*0.8) * time.Second
	a.refreshAt = now.Add(refreshDuration)

	return map[string]string{"Authorization": "Bearer " + a.cachedToken}, nil
}

// fetchToken performs the OAuth 2.0 client_credentials grant request.
func (a *OAuthClientCredentialsAuth) fetchToken(ctx context.Context) (string, int, error) {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {a.ClientID},
		"client_secret": {a.ClientSecret},
	}
	if len(a.Scopes) > 0 {
		data.Set("scope", strings.Join(a.Scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("parsing token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("token response missing access_token")
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

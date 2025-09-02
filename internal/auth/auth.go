package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

const (
	AuthorizeURL = "https://www.dropbox.com/oauth2/authorize"
	TokenURL     = "https://api.dropboxapi.com/oauth2/token" // #nosec G101 - This is a URL, not a credential
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func StartOAuthFlow(config OAuthConfig) (*AuthResult, error) {
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  AuthorizeURL,
			TokenURL: TokenURL,
		},
		RedirectURL: redirectURI,
		Scopes:      []string{},
	}

	authURL := oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("token_access_type", "offline"),
	)

	resultChan := make(chan *AuthResult, 1)
	errorChan := make(chan error, 1)

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}

			queryState := r.URL.Query().Get("state")
			if queryState != state {
				errorChan <- fmt.Errorf("state mismatch")
				http.Error(w, "State mismatch", http.StatusBadRequest)
				return
			}

			code := r.URL.Query().Get("code")
			if code == "" {
				errorMsg := r.URL.Query().Get("error_description")
				if errorMsg == "" {
					errorMsg = r.URL.Query().Get("error")
				}
				errorChan <- fmt.Errorf("authorization failed: %s", errorMsg)
				http.Error(w, "Authorization failed", http.StatusBadRequest)
				return
			}

			ctx := context.Background()
			token, err := oauth2Config.Exchange(ctx, code)
			if err != nil {
				errorChan <- fmt.Errorf("token exchange failed: %w", err)
				http.Error(w, "Token exchange failed", http.StatusInternalServerError)
				return
			}

			result := &AuthResult{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresAt:    token.Expiry,
			}

			resultChan <- result

			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authentication Successful</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #4CAF50; }
    </style>
</head>
<body>
    <h1 class="success">Authentication Successful!</h1>
    <p>You can now close this window and return to your terminal.</p>
    <script>setTimeout(function(){ window.close(); }, 3000);</script>
</body>
</html>`)
		}),
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errorChan <- err
		}
	}()

	if err := browser.OpenURL(authURL); err != nil {
		return nil, fmt.Errorf("failed to open browser: %w", err)
	}

	select {
	case result := <-resultChan:
		server.Close()
		return result, nil
	case err := <-errorChan:
		server.Close()
		return nil, err
	case <-time.After(5 * time.Minute):
		server.Close()
		return nil, fmt.Errorf("authentication timeout")
	}
}

func RefreshToken(config OAuthConfig, refreshToken string) (*AuthResult, error) {
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  AuthorizeURL,
			TokenURL: TokenURL,
		},
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	ctx := context.Background()
	newToken, err := oauth2Config.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return &AuthResult{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		ExpiresAt:    newToken.Expiry,
	}, nil
}

func ValidateToken(accessToken string) error {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.dropboxapi.com/2/check/user", http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid token: status %d", resp.StatusCode)
	}

	return nil
}

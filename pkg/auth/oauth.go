package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agy-tools/pkg/types"
)

// GenerateAuthURL returns Google OAuth 2.0 authorization URL
func GenerateAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", types.GoogleClientID)
	params.Set("redirect_uri", types.OAuthRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(types.OAuthScopes, " "))
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	if state != "" {
		params.Set("state", state)
	}
	return fmt.Sprintf("%s?%s", types.GoogleAuthURL, params.Encode())
}

// ExchangeCode exchanges an authorization code for TokenData
func ExchangeCode(code string) (*types.TokenData, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", types.GoogleClientID)
	data.Set("client_secret", types.GoogleClientSecret)
	data.Set("redirect_uri", types.OAuthRedirectURI)
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(types.GoogleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		IDToken      string `json:"id_token"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	return &types.TokenData{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresAt:    time.Now().UnixMilli() + (res.ExpiresIn * 1000),
		TokenType:    res.TokenType,
		IDToken:      res.IDToken,
	}, nil
}

// RefreshTokens exchanges a refresh token for new access tokens
func RefreshTokens(refreshToken string) (*types.TokenData, error) {
	data := url.Values{}
	data.Set("client_id", types.GoogleClientID)
	data.Set("client_secret", types.GoogleClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	resp, err := http.PostForm(types.GoogleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh tokens: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		IDToken      string `json:"id_token"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	newRefreshToken := refreshToken
	if res.RefreshToken != "" {
		newRefreshToken = res.RefreshToken
	}

	return &types.TokenData{
		AccessToken:  res.AccessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    time.Now().UnixMilli() + (res.ExpiresIn * 1000),
		TokenType:    res.TokenType,
		IDToken:      res.IDToken,
	}, nil
}

// FetchUserInfo retrieves email and name from Google UserInfo endpoint
func FetchUserInfo(accessToken string) (email, name string, err error) {
	req, err := http.NewRequest("GET", types.GoogleUserInfoURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to fetch user info: status %d", resp.StatusCode)
	}

	var data struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", err
	}

	return data.Email, data.Name, nil
}

// FetchProjectIDAndTier discovers the Antigravity user tier and project ID from loadCodeAssist
func FetchProjectIDAndTier(accessToken string) (projectID, tier string) {
	endpoints := []string{
		"https://cloudcode-pa.googleapis.com",
		"https://daily-cloudcode-pa.sandbox.googleapis.com",
		"https://autopush-cloudcode-pa.sandbox.googleapis.com",
	}

	payload := []byte(`{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`)

	client := &http.Client{Timeout: 3 * time.Second}

	for _, base := range endpoints {
		urlStr := base + "/v1internal:loadCodeAssist"
		req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var data struct {
			CloudAICOMPANIONProject any `json:"cloudaicompanionProject"`
			PaidTier                struct {
				ID string `json:"id"`
			} `json:"paidTier"`
			CurrentTier struct {
				ID string `json:"id"`
			} `json:"currentTier"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			continue
		}

		var proj string
		switch v := data.CloudAICOMPANIONProject.(type) {
		case string:
			proj = v
		case map[string]interface{}:
			if idStr, ok := v["id"].(string); ok {
				proj = idStr
			}
		}

		t := data.PaidTier.ID
		if t == "" {
			t = data.CurrentTier.ID
		}
		if t == "" {
			t = "FREE"
		}

		if proj != "" {
			return proj, t
		}
	}

	return "", "FREE"
}

// StartOAuthFlow starts a local web server to capture OAuth redirect callback
func StartOAuthFlow(ctx context.Context) (authURL string, resultChan <-chan *types.TokenData, errChan <-chan error) {
	resCh := make(chan *types.TokenData, 1)
	errCh := make(chan error, 1)

	state := fmt.Sprintf("%d", time.Now().UnixNano())
	authURL = GenerateAuthURL(state)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", types.OAuthRedirectPort),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errStr := r.URL.Query().Get("error")
			if errStr == "" {
				errStr = "missing authorization code"
			}
			http.Error(w, "OAuth Error: "+errStr, http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth error: %s", errStr)
			return
		}

		tokens, err := ExchangeCode(code)
		if err != nil {
			http.Error(w, "Token exchange failed: "+err.Error(), http.StatusInternalServerError)
			errCh <- err
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<html><body style="font-family:sans-serif;text-align:center;padding:50px;background:#0f172a;color:#fff;">
			<h2>🎉 授权成功！</h2>
			<p>Google 账户已成功连接至 Antigravity。您可以关闭此页面返回控制台。</p>
			<script>setTimeout(() => window.close(), 2500);</script>
		</body></html>`)

		resCh <- tokens

		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = server.Shutdown(context.Background())
		}()
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return authURL, resCh, errCh
}

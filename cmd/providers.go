package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	xgithub "golang.org/x/oauth2/github"
	xgoogle "golang.org/x/oauth2/google"
)

var yandexEndpoint = oauth2.Endpoint{
	AuthURL:  "https://oauth.yandex.ru/authorize",
	TokenURL: "https://oauth.yandex.ru/token",
}

type UserInfo struct {
	Sub  string
	Name string
}

type fetchUserInfoFunc func(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*UserInfo, error)

type providerConfig struct {
	oauth2Config *oauth2.Config
	fetchUser    fetchUserInfoFunc
}

func buildProviders(cfg *Config) map[string]*providerConfig {
	providers := map[string]*providerConfig{}

	if cfg.YandexClientID != "" && cfg.YandexClientSecret != "" {
		providers["yandex"] = &providerConfig{
			oauth2Config: &oauth2.Config{
				ClientID:     cfg.YandexClientID,
				ClientSecret: cfg.YandexClientSecret,
				Endpoint:     yandexEndpoint,
				RedirectURL:  fmt.Sprintf("https://auth.%s/with/yandex/callback", cfg.Domain),
				Scopes:       []string{"login:email", "login:info"},
			},
			fetchUser: fetchYandexUser,
		}
	}

	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		providers["google"] = &providerConfig{
			oauth2Config: &oauth2.Config{
				ClientID:     cfg.GoogleClientID,
				ClientSecret: cfg.GoogleClientSecret,
				Endpoint:     xgoogle.Endpoint,
				RedirectURL:  fmt.Sprintf("https://auth.%s/with/google/callback", cfg.Domain),
				Scopes:       []string{"openid", "email", "profile"},
			},
			fetchUser: fetchGoogleUser,
		}
	}

	if cfg.GithubClientID != "" && cfg.GithubClientSecret != "" {
		providers["github"] = &providerConfig{
			oauth2Config: &oauth2.Config{
				ClientID:     cfg.GithubClientID,
				ClientSecret: cfg.GithubClientSecret,
				Endpoint:     xgithub.Endpoint,
				RedirectURL:  fmt.Sprintf("https://auth.%s/with/github/callback", cfg.Domain),
				Scopes:       []string{"read:user", "user:email"},
			},
			fetchUser: fetchGithubUser,
		}
	}

	return providers
}

func fetchYandexUser(ctx context.Context, _ *oauth2.Config, token *oauth2.Token) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://login.yandex.ru/info?format=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "OAuth "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	return &UserInfo{
		Sub:  firstNonEmpty(strVal(raw["default_email"]), strVal(raw["login"])),
		Name: strVal(raw["name"]),
	}, nil
}

func fetchGoogleUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*UserInfo, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	return &UserInfo{
		Sub:  strVal(raw["email"]),
		Name: strVal(raw["name"]),
	}, nil
}

func fetchGithubUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*UserInfo, error) {
	client := cfg.Client(ctx, token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "oauth-service")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	return &UserInfo{
		Sub:  firstNonEmpty(strVal(raw["email"]), strVal(raw["login"])),
		Name: strVal(raw["name"]),
	}, nil
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}


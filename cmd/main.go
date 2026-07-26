package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
)

const sessionName = "session"

var (
	cfg       *Config
	providers map[string]*providerConfig
	store     *sessions.CookieStore
)

func main() {
	cfg = loadConfig()
	providers = buildProviders(cfg)
	store = sessions.NewCookieStore([]byte(cfg.SecretKey))

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int((30 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /with/{provider}", handleLogin)
	mux.HandleFunc("GET /with/{provider}/callback", handleCallback)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	canonicalHost := "auth." + cfg.Domain
	if r.Host != canonicalHost {
		target := *r.URL
		target.Scheme = "https"
		target.Host = canonicalHost
		http.Redirect(w, r, target.String(), http.StatusFound)
		return
	}

	provider := r.PathValue("provider")
	pc, ok := providers[provider]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusNotFound)
		return
	}

	next := r.URL.Query().Get("next")
	if next == "" {
		next = "https://" + cfg.Domain + "/"
	}

	state, err := randomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sess, _ := store.Get(r, sessionName)
	sess.Values["state"] = state
	sess.Values["next"] = next
	if err := sess.Save(r, w); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authURL := pc.oauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	pc, ok := providers[provider]
	if !ok {
		http.Error(w, "Unknown provider", http.StatusNotFound)
		return
	}

	sess, _ := store.Get(r, sessionName)

	wantState, _ := sess.Values["state"].(string)
	gotState := r.URL.Query().Get("state")
	if wantState == "" || gotState == "" || gotState != wantState {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	token, err := pc.oauth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}

	userInfo, err := pc.fetchUser(ctx, pc.oauth2Config, token)
	if err != nil || userInfo.Sub == "" {
		http.Error(w, "failed to fetch user info", http.StatusBadGateway)
		return
	}

	claims := jwt.MapClaims{
		"sub": userInfo.Sub,
		"name": userInfo.Name,
		"exp": time.Now().Add(jwtExpiration).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := jwtToken.SignedString(cfg.JWTSecret)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	next, _ := sess.Values["next"].(string)
	if next == "" {
		next = "https://" + cfg.Domain + "/"
	}
	delete(sess.Values, "next")
	delete(sess.Values, "state")
	if err := sess.Save(r, w); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    signed,
		Domain:   "." + cfg.Domain,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	if u, err := url.Parse(next); err == nil {
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	http.Redirect(w, r, "https://"+cfg.Domain+"/", http.StatusFound)
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}


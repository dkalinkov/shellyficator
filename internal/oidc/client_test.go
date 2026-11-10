package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestClientValidatesOIDCAndUsesPKCE(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const nonce = "login-nonce"
	verifier := strings.Repeat("v", 43)
	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "userinfo_endpoint": issuer + "/userinfo", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			id, secret, ok := r.BasicAuth()
			if !ok || id != "test-client" || secret != "client-secret" || r.Form.Get("code_verifier") != verifier || r.Form.Get("redirect_uri") != "https://app.example.com/auth/oidc/callback" {
				t.Error("invalid token exchange or missing PKCE")
				w.WriteHeader(400)
				return
			}
			code := r.Form.Get("code")
			if code == "token-error" {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			claims := jwt.MapClaims{"iss": issuer, "sub": "stable-subject", "aud": "test-client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce}
			signingKey := key
			switch code {
			case "wrong-issuer":
				claims["iss"] = "https://other.example.com"
			case "wrong-audience":
				claims["aud"] = "other-client"
			case "expired":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "wrong-nonce":
				claims["nonce"] = "another-login"
			case "missing-subject":
				delete(claims, "sub")
			case "wrong-signature":
				signingKey = otherKey
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = "test-key"
			raw, err := token.SignedString(signingKey)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			result := map[string]any{"access_token": code, "token_type": "Bearer", "expires_in": 3600, "id_token": raw}
			if code == "missing-id-token" {
				delete(result, "id_token")
			}
			_ = json.NewEncoder(w).Encode(result)
		case "/userinfo":
			subject := "stable-subject"
			if r.Header.Get("Authorization") == "Bearer mismatched-userinfo" {
				subject = "other-subject"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sub": subject, "email": "operator@example.com", "email_verified": true})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	issuer = server.URL
	client, err := New(Config{Issuer: issuer, ClientID: "test-client", ClientSecret: "client-secret", RedirectURL: "https://app.example.com/auth/oidc/callback"})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := client.AuthorizationURL(t.Context(), "browser-state", nonce, verifier)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorization)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	challenge := sha256.Sum256([]byte(verifier))
	if query.Get("state") != "browser-state" || query.Get("nonce") != nonce || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) || strings.Contains(authorization, "client-secret") {
		t.Fatal("authorization URL lost browser binding, nonce or PKCE")
	}
	for _, code := range []string{"valid", "wrong-issuer", "wrong-audience", "expired", "wrong-nonce", "missing-subject", "wrong-signature", "missing-id-token", "mismatched-userinfo", "token-error"} {
		t.Run(code, func(t *testing.T) {
			identity, err := client.Exchange(t.Context(), code, nonce, verifier)
			if code == "valid" {
				if err != nil || identity.Issuer != issuer || identity.Subject != "stable-subject" || identity.Email != "operator@example.com" || !identity.EmailVerified {
					t.Fatalf("valid login rejected: %v", err)
				}
			} else if err == nil {
				t.Fatal("invalid OIDC response accepted")
			}
		})
	}
}

func TestConfigurationRequiresCompleteSecureURLs(t *testing.T) {
	if client, err := New(Config{}); err != nil || client != nil {
		t.Fatal("empty configuration enabled OIDC")
	}
	base := Config{Issuer: "https://auth.example.com", ClientID: "client", ClientSecret: "sensitive-secret-value", RedirectURL: "https://app.example.com/auth/oidc/callback"}
	for _, change := range []func(*Config){
		func(c *Config) { c.Issuer = "http://auth.example.com" },
		func(c *Config) { c.RedirectURL = "http://app.example.com/callback" },
		func(c *Config) { c.Issuer = "https://user:password@auth.example.com" },
		func(c *Config) { c.RedirectURL += "?untrusted=value" },
		func(c *Config) { c.ClientID = "" },
		func(c *Config) { c.ClientSecret = "" },
	} {
		config := base
		change(&config)
		if _, err := New(config); err == nil || strings.Contains(err.Error(), base.ClientSecret) {
			t.Fatal("invalid configuration accepted")
		}
	}
}

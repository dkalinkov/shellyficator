package oidc

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"shellyficator/internal/domain"
)

type Config struct {
	Issuer, ClientID, ClientSecret, RedirectURL string
}

type Client struct {
	config   Config
	http     *http.Client
	mu       sync.Mutex
	provider *gooidc.Provider
}

// New validates configuration without contacting the provider. Password login
// and application startup remain available during an identity-provider outage.
func New(config Config) (*Client, error) {
	if config == (Config{}) {
		return nil, nil
	}
	if config.ClientID == "" || config.ClientSecret == "" || !validURL(config.Issuer) || !validURL(config.RedirectURL) {
		return nil, errors.New("configure SHELLY_OIDC_ISSUER, SHELLY_OIDC_CLIENT_ID, SHELLY_OIDC_CLIENT_SECRET and SHELLY_OIDC_REDIRECT_URL; URLs require HTTPS (HTTP is allowed on loopback)")
	}
	return &Client{config: config, http: &http.Client{Timeout: 10 * time.Second}}, nil
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}

func (c *Client) discover(ctx context.Context) (*gooidc.Provider, oauth2.Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider == nil {
		provider, err := gooidc.NewProvider(gooidc.ClientContext(ctx, c.http), c.config.Issuer)
		if err != nil {
			return nil, oauth2.Config{}, err
		}
		c.provider = provider
	}
	endpoint := c.provider.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInHeader
	return c.provider, oauth2.Config{ClientID: c.config.ClientID, ClientSecret: c.config.ClientSecret, Endpoint: endpoint, RedirectURL: c.config.RedirectURL, Scopes: []string{gooidc.ScopeOpenID, "profile", "email"}}, nil
}

func (c *Client) AuthorizationURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	_, config, err := c.discover(ctx)
	if err != nil {
		return "", err
	}
	return config.AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

func (c *Client) Exchange(ctx context.Context, code, nonce, verifier string) (domain.ExternalIdentity, error) {
	provider, config, err := c.discover(ctx)
	if err != nil {
		return domain.ExternalIdentity{}, err
	}
	ctx = gooidc.ClientContext(ctx, c.http)
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return domain.ExternalIdentity{}, err
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return domain.ExternalIdentity{}, errors.New("provider did not return an ID token")
	}
	id, err := provider.Verifier(&gooidc.Config{ClientID: c.config.ClientID}).Verify(ctx, raw)
	if err != nil {
		return domain.ExternalIdentity{}, err
	}
	if id.Subject == "" || nonce == "" || subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(nonce)) != 1 {
		return domain.ExternalIdentity{}, errors.New("invalid identity or login nonce")
	}
	if id.AccessTokenHash != "" {
		if err := id.VerifyAccessToken(token.AccessToken); err != nil {
			return domain.ExternalIdentity{}, err
		}
	}
	info, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return domain.ExternalIdentity{}, err
	}
	if info.Subject != id.Subject {
		return domain.ExternalIdentity{}, errors.New("provider returned conflicting identities")
	}
	return domain.ExternalIdentity{Issuer: id.Issuer, Subject: id.Subject, Email: info.Email, EmailVerified: info.EmailVerified}, nil
}

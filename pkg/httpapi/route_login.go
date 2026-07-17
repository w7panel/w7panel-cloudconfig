package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	defaultOIDCDiscoveryURL = "http://w7panel-offline.default.svc:8000/.well-known/openid-configuration"
	defaultOIDCRedirectURL  = "http://127.0.0.1:3000/callback"
	defaultOIDCClientID     = "default"
)

var defaultOIDCScopes = []string{"openid", "profile"}

type oidcDiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type oidcProviderState struct {
	discovery *oidcDiscoveryDocument
	verifier  *oidc.IDTokenVerifier
}

type idTokenClaims struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Role              string `json:"role"`
	IsFounder         bool   `json:"is_founder"`
}

func (c *idTokenClaims) username() string {
	if c == nil {
		return ""
	}
	if value := strings.TrimSpace(c.PreferredUsername); value != "" {
		return value
	}
	return strings.TrimSpace(c.Subject)
}

func (c *idTokenClaims) canManageConfig() bool {
	if c == nil {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(c.Role))
	return c.IsFounder || role == "founder" || role == "super"
}

type loginRequest struct {
	Code string `json:"code" binding:"required"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Username    string `json:"username"`
}

type loginConfigResponse struct {
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	ClientID              string   `json:"client_id"`
	RedirectURI           string   `json:"redirect_uri"`
	Scope                 string   `json:"scope"`
	ResponseType          string   `json:"response_type"`
	Scopes                []string `json:"scopes"`
}

func (s *Server) loginConfig(c *gin.Context) {
	state, err := s.loadOIDCProvider(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loginConfigResponse{
		AuthorizationEndpoint: state.discovery.AuthorizationEndpoint,
		ClientID:              s.oidcClientID(),
		RedirectURI:           s.oidcRedirectURL(),
		Scope:                 strings.Join(defaultOIDCScopes, " "),
		ResponseType:          "code",
		Scopes:                append([]string(nil), defaultOIDCScopes...),
	})
}

func (s *Server) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "code is required"})
		return
	}
	rawToken, token, claims, err := s.exchangeOIDCCode(c.Request.Context(), request.Code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	if !claims.canManageConfig() {
		c.JSON(http.StatusForbidden, gin.H{"message": "insufficient permissions"})
		return
	}
	expiresIn := int64(time.Until(token.Expiry).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}
	c.JSON(http.StatusOK, loginResponse{
		AccessToken: rawToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Username:    claims.username(),
	})
}

func (s *Server) exchangeOIDCCode(ctx context.Context, code string) (string, *oidc.IDToken, *idTokenClaims, error) {
	state, err := s.loadOIDCProvider(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	oauthConfig := oauth2.Config{
		ClientID:    s.oidcClientID(),
		RedirectURL: s.oidcRedirectURL(),
		Endpoint: oauth2.Endpoint{
			TokenURL:  state.discovery.TokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	token, err := oauthConfig.Exchange(s.oidcContext(ctx), code)
	if err != nil {
		return "", nil, nil, fmt.Errorf("exchange oidc code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return "", nil, nil, errors.New("oidc token response missing id_token")
	}
	idToken, claims, err := verifyRawIDToken(ctx, state.verifier, rawIDToken)
	if err != nil {
		return "", nil, nil, err
	}
	return rawIDToken, idToken, claims, nil
}

func (s *Server) verifyIDToken(ctx context.Context, rawToken string) (*idTokenClaims, error) {
	state, err := s.loadOIDCProvider(ctx)
	if err != nil {
		return nil, err
	}
	_, claims, err := verifyRawIDToken(ctx, state.verifier, rawToken)
	return claims, err
}

func verifyRawIDToken(ctx context.Context, verifier *oidc.IDTokenVerifier, rawToken string) (*oidc.IDToken, *idTokenClaims, error) {
	token, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, nil, fmt.Errorf("verify id token: %w", err)
	}
	var claims idTokenClaims
	if err := token.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("decode id token claims: %w", err)
	}
	if claims.username() == "" {
		return nil, nil, errors.New("id token missing username claim")
	}
	return token, &claims, nil
}

func (s *Server) loadOIDCProvider(ctx context.Context) (*oidcProviderState, error) {
	s.oidcMu.Lock()
	defer s.oidcMu.Unlock()
	if s.oidcState != nil {
		return s.oidcState, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.oidcDiscoveryURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("build oidc discovery request: %w", err)
	}
	response, err := s.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("request oidc discovery: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read oidc discovery response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc discovery returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var discovery oidcDiscoveryDocument
	if err := json.Unmarshal(body, &discovery); err != nil {
		return nil, fmt.Errorf("decode oidc discovery response: %w", err)
	}
	if discovery.Issuer == "" || discovery.AuthorizationEndpoint == "" || discovery.TokenEndpoint == "" || discovery.JWKSURI == "" {
		return nil, errors.New("oidc discovery response is incomplete")
	}
	keySet := oidc.NewRemoteKeySet(s.oidcContext(context.Background()), discovery.JWKSURI)
	state := &oidcProviderState{
		discovery: &discovery,
		verifier: oidc.NewVerifier(discovery.Issuer, keySet, &oidc.Config{
			ClientID: s.oidcClientID(),
		}),
	}
	s.oidcState = state
	return state, nil
}

func configAuthorizationToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("missing or invalid Authorization-config header")
	}
	return parts[1], nil
}

func (s *Server) oidcDiscoveryURL() string {
	if value := strings.TrimSpace(s.OIDCDiscoveryURL); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("OIDC_DISCOVERY_URL")); value != "" {
		return value
	}
	return defaultOIDCDiscoveryURL
}

func (s *Server) oidcRedirectURL() string {
	if value := strings.TrimSpace(s.OIDCRedirectURL); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("OIDC_REDIRECT_URL")); value != "" {
		return value
	}
	return defaultOIDCRedirectURL
}

func (s *Server) oidcClientID() string {
	if value := strings.TrimSpace(s.OIDCClientID); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID")); value != "" {
		return value
	}
	return defaultOIDCClientID
}

func (s *Server) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *Server) oidcContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, s.httpClient())
}

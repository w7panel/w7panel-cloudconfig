package httpapi

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type testOIDCIssuer struct {
	server     *httptest.Server
	clientID   string
	privateKey *rsa.PrivateKey
}

func newTestOIDCIssuer(t *testing.T) *testOIDCIssuer {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := &testOIDCIssuer{clientID: "cloudconfig-test", privateKey: privateKey}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(t, w, map[string]any{
			"issuer":                 issuer.server.URL,
			"authorization_endpoint": issuer.server.URL + "/authorize",
			"token_endpoint":         issuer.server.URL + "/token",
			"jwks_uri":               issuer.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(t, w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &privateKey.PublicKey,
			KeyID:     "test-key",
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse token request: %v", err)
		}
		code := r.Form.Get("code")
		if code == "invalid" {
			http.Error(w, "invalid code", http.StatusUnauthorized)
			return
		}
		role := "super"
		if code == "normal" {
			role = "normal"
		}
		rawToken := issuer.signToken(t, role, false, time.Now().Add(time.Hour))
		writeTestJSON(t, w, map[string]any{
			"access_token": "upstream-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     rawToken,
		})
	})
	issuer.server = httptest.NewServer(mux)
	t.Cleanup(issuer.server.Close)
	return issuer
}

func (i *testOIDCIssuer) signToken(t *testing.T, role string, founder bool, expiry time.Time) string {
	return i.signTokenForAudience(t, role, founder, expiry, i.clientID)
}

func (i *testOIDCIssuer) signTokenForAudience(t *testing.T, role string, founder bool, expiry time.Time, audience string) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: i.privateKey}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(map[string]any{
		"iss":                i.server.URL,
		"aud":                audience,
		"sub":                "user-1",
		"preferred_username": "alice",
		"role":               role,
		"is_founder":         founder,
		"iat":                time.Now().Add(-time.Minute).Unix(),
		"exp":                expiry.Unix(),
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (i *testOIDCIssuer) newServer() *Server {
	return &Server{
		OIDCDiscoveryURL: i.server.URL + "/.well-known/openid-configuration",
		OIDCRedirectURL:  "http://127.0.0.1:3000/callback",
		OIDCClientID:     i.clientID,
		HTTPClient:       i.server.Client(),
	}
}

func TestLoginConfigAndExchange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	issuer := newTestOIDCIssuer(t)
	server := issuer.newServer()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/cloudconfig-api/v1/login/config", nil)
	server.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login config returned %d: %s", recorder.Code, recorder.Body.String())
	}
	var config loginConfigResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	if config.ClientID != issuer.clientID || config.AuthorizationEndpoint != issuer.server.URL+"/authorize" {
		t.Fatalf("unexpected login config: %#v", config)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/cloudconfig-api/v1/login", strings.NewReader(`{"code":"good"}`))
	request.Header.Set("Content-Type", "application/json")
	server.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", recorder.Code, recorder.Body.String())
	}
	var response loginResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AccessToken == "" || response.TokenType != "Bearer" || response.Username != "alice" || response.ExpiresIn <= 0 {
		t.Fatalf("unexpected login response: %#v", response)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/cloudconfig-api/v1/login", strings.NewReader(`{"code":"normal"}`))
	request.Header.Set("Content-Type", "application/json")
	server.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("normal user login returned %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/cloudconfig-api/v1/login", strings.NewReader(`{"code":"invalid"}`))
	request.Header.Set("Content-Type", "application/json")
	server.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("invalid code returned %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthorizationConfigMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	issuer := newTestOIDCIssuer(t)
	server := issuer.newServer()
	router := gin.New()
	router.Use(server.authMiddleware())
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{name: "super", header: "Authorization-config", value: "Bearer " + issuer.signToken(t, "super", false, time.Now().Add(time.Hour)), want: http.StatusNoContent},
		{name: "founder claim", header: "Authorization-config", value: "Bearer " + issuer.signToken(t, "normal", true, time.Now().Add(time.Hour)), want: http.StatusNoContent},
		{name: "normal forbidden", header: "Authorization-config", value: "Bearer " + issuer.signToken(t, "normal", false, time.Now().Add(time.Hour)), want: http.StatusForbidden},
		{name: "expired", header: "Authorization-config", value: "Bearer " + issuer.signToken(t, "super", false, time.Now().Add(-time.Hour)), want: http.StatusUnauthorized},
		{name: "wrong audience", header: "Authorization-config", value: "Bearer " + issuer.signTokenForAudience(t, "super", false, time.Now().Add(time.Hour), "another-client"), want: http.StatusUnauthorized},
		{name: "standard header rejected", header: "Authorization", value: "Bearer " + issuer.signToken(t, "super", false, time.Now().Add(time.Hour)), want: http.StatusUnauthorized},
		{name: "missing", header: "Authorization-config", value: "", want: http.StatusUnauthorized},
		{name: "malformed", header: "Authorization-config", value: "not-a-token", want: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set(test.header, test.value)
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("got %d, want %d: %s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func TestConfigAuthorizationToken(t *testing.T) {
	for _, header := range []string{"", "token", "Basic token", "Bearer", "Bearer a b"} {
		if _, err := configAuthorizationToken(header); err == nil {
			t.Errorf("expected %q to fail", header)
		}
	}
	if token, err := configAuthorizationToken("bearer token"); err != nil || token != "token" {
		t.Fatalf("unexpected parse result token=%q err=%v", token, err)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/bytedance/sonic"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/cast"
	"golang.org/x/oauth2"
)

func (cluster *Cluster) oauth2Config() *oauth2.Config {
	provider := cluster.Provider()
	if provider == nil {
		return nil
	}
	endpoint := provider.Endpoint()
	// Public clients (no secret) must send credentials in the request body
	// per RFC 6749 §2.3.1; some IdPs reject the otherwise auto-detected
	// HTTP Basic header when its password component is empty.
	if cluster.PKCE && cluster.Client_Secret == "" {
		endpoint.AuthStyle = oauth2.AuthStyleInParams
	}
	return &oauth2.Config{
		ClientID:     cluster.Client_ID,
		ClientSecret: cluster.Client_Secret,
		Endpoint:     endpoint,
		Scopes:       cluster.Scopes,
		RedirectURL:  cluster.Redirect_URI,
	}
}

func (config *Config) handleIndex(w http.ResponseWriter, r *http.Request) {

	if len(config.Clusters) == 1 && r.URL.String() == config.Web_Path_Prefix {
		http.Redirect(w, r, path.Join(config.Web_Path_Prefix, "login", config.Clusters[0].Name), http.StatusSeeOther)
	} else {
		renderIndex(w, config)
	}
}

func (cluster *Cluster) handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling login-uri for: %s", cluster.Name)

	cfg := cluster.oauth2Config()
	if cfg == nil {
		cluster.renderHTMLError(w, "Identity provider not ready, please retry shortly", http.StatusServiceUnavailable)
		log.Printf("handleLogin: OIDC provider for %q not initialized yet", cluster.Name)
		return
	}

	state := generateState()
	authOpts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if cluster.PKCE {
		verifier := oauth2.GenerateVerifier()
		cluster.Sessions.put(state, verifier)
		authOpts = append(authOpts, oauth2.S256ChallengeOption(verifier))
	}

	authCodeURL := cfg.AuthCodeURL(state, authOpts...)
	if cluster.Connector_ID != "" {
		log.Printf("Using dex connector with id %#q", cluster.Connector_ID)
		authCodeURL = fmt.Sprintf("%s&connector_id=%s", authCodeURL, cluster.Connector_ID)
	}
	log.Printf("Redirecting post-login to: %s", authCodeURL)
	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

func (cluster *Cluster) handleCallback(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		token    *oauth2.Token
		IdpCaPem string
	)

	// An error message to that presented to the user
	userErrorMsg := "Invalid token request"

	log.Printf("Handling callback for: %s", cluster.Name)

	ctx := oidc.ClientContext(r.Context(), cluster.Client)
	oauth2Config := cluster.oauth2Config()
	verifier := cluster.Verifier()
	if oauth2Config == nil || verifier == nil {
		cluster.renderHTMLError(w, "Identity provider not ready, please retry shortly", http.StatusServiceUnavailable)
		log.Printf("handleCallback: OIDC provider for %q not initialized yet", cluster.Name)
		return
	}
	switch r.Method {
	case "GET":
		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
			log.Printf("handleCallback: request error. error: %s, error_description: %s", errMsg, r.FormValue("error_description"))
			return
		}
		code := r.FormValue("code")
		if code == "" {
			cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
			log.Printf("handleCallback: no code in request: %q", r.Form)
			return
		}
		state := r.FormValue("state")
		if state == "" {
			cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
			log.Printf("handleCallback: missing state in callback")
			return
		}

		exchangeOpts := []oauth2.AuthCodeOption{}
		if cluster.PKCE {
			verifier, ok := cluster.Sessions.take(state)
			if !ok {
				cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
				log.Printf("handleCallback: unknown or expired state %q", state)
				return
			}
			exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(verifier))
		}
		token, err = oauth2Config.Exchange(ctx, code, exchangeOpts...)
	case "POST":
		// Form request from frontend to refresh a token.
		refresh := r.FormValue("refresh_token")
		if refresh == "" {
			cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
			log.Printf("handleCallback: no refresh_token in request: %q", r.Form)
			return
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(ctx, t).Token()
	default:
		// Return non-HTML error for non GET/POST requests which probably wasn't executed by browser
		http.Error(w, fmt.Sprintf("Method not implemented: %s", r.Method), http.StatusBadRequest)
		return
	}

	if err != nil {
		cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
		log.Printf("handleCallback: failed to get token: %v", err)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
		log.Printf("handleCallback: no id_token in response: %v", token)
		return
	}

	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
		log.Printf("handleCallback: failed to verify ID token: %q, err: %v", rawIDToken, err)
		return
	}
	var claims json.RawMessage
	if err = idToken.Claims(&claims); err != nil {
		cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
		log.Printf("handleCallback: failed to unmarshal json payload of ID token into claims: %v", err)
		return
	}

	var claimsObj interface{}
	if err = sonic.Unmarshal([]byte(claims), &claimsObj); err != nil {
		cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
		log.Printf("handleCallback: failed to parse claims json: %v", err)
		return
	}
	indentedClaims, err := sonic.ConfigStd.MarshalIndent(claimsObj, "", "  ")
	if err != nil {
		cluster.renderHTMLError(w, userErrorMsg, http.StatusBadRequest)
		log.Printf("handleCallback: failed to indent json:  %v", err)
		return
	}

	if cluster.Config.IDP_Ca_Pem != "" {
		IdpCaPem = cluster.Config.IDP_Ca_Pem
	} else if cluster.Config.IDP_Ca_Pem_File != "" {
		content, err := os.ReadFile(cluster.Config.IDP_Ca_Pem_File)
		if err != nil {
			log.Fatalf("Failed to load CA from file %s, %s", cluster.Config.IDP_Ca_Pem_File, err)
		}
		IdpCaPem = cast.ToString(content)
	}

	cluster.renderToken(w, rawIDToken, token.RefreshToken,
		cluster.Config.IDP_Ca_URI,
		IdpCaPem,
		cluster.Config.Logo_Uri,
		cluster.Config.Web_Path_Prefix,
		cluster.Config.Kubectl_Version,
		indentedClaims)
}

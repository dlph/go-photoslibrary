package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"golang.org/x/exp/slog"
	"golang.org/x/oauth2"
)

func TestAuth(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("/o/oauth2/auth", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI, err := url.ParseRequestURI(r.RequestURI)
		if err != nil {
			t.Fatal(err)
		}
		redirectURI := requestURI.Query().Get("redirect_uri")
		q := make(url.Values)
		q.Add("code", "12345")
		q.Add("state", requestURI.Query().Get("state"))
		q.Add("scope", requestURI.Query().Get("scope"))

		redirectURL, err := url.Parse(redirectURI)
		if err != nil {
			t.Fatal(err)
		}

		redirectURL.RawQuery = q.Encode()

		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	}))
	mux.Handle("/token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed reading body", "error", err)
			return
		}
		slog.DebugContext(r.Context(), "handling token response", "body", body)

		token := &oauth2.Token{
			AccessToken:  "54321",
			TokenType:    "Bearer",
			RefreshToken: "r54321",
			Expiry:       time.Now().Add(5 * time.Hour),
		}

		w.Header().Add("Content-Type", mime.FormatMediaType("application/json", map[string]string{}))
		if err := json.NewEncoder(w).Encode(token); err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
	}))
	mux.Handle("/device/code", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testServ := httptest.NewServer(mux)
	defer testServ.Close()

	redirectURL := "http://localhost:44185"
	config := &oauth2.Config{
		ClientID:     "<YOUR_CLIENT_ID>.apps.googleusercontent.com",
		ClientSecret: "<YOUR_CLIENT_SECRET>",
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/photoslibrary.readonly",
			"https://www.googleapis.com/auth/photoslibrary.appendonly",
			"https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata",
			"https://www.googleapis.com/auth/photoslibrary.edit.appcreateddata",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:       fmt.Sprintf("%s/o/oauth2/auth", testServ.URL),
			TokenURL:      fmt.Sprintf("%s/token", testServ.URL),
			DeviceAuthURL: fmt.Sprintf("%s/device/code", testServ.URL),
			AuthStyle:     oauth2.AuthStyleInParams,
		},
	}

	client, err := NewClient(ctx, config, WithAuthCodeURLHandler(func(authCodeURL string) {
		slog.DebugContext(ctx, "received authurl", "authCodeURL", authCodeURL)
		resp, err := testServ.Client().Get(authCodeURL)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp
	}))
	if err != nil {
		t.Fatal(err)
	}

	if client == nil {
		t.Fatal("nil client")
	}
}

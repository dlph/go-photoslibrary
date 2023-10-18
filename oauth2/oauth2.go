package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/exp/slog"
	"golang.org/x/oauth2"
)

const (
	stateURLQueryKey = "state"
	stateSize        = 10
)

type Config struct {
	authCodeURLHandlerFn func(authCodeURL string)
}

type Option func(*Config)

func WithAuthCodeURLHandler(fn func(authCodeURL string)) Option {
	return func(c *Config) {
		c.authCodeURLHandlerFn = fn
	}
}

// NewClient creates a new http client which handles authorization/authentication
// Your credentials should be obtained from the Google
// Developer Console (https://console.developers.google.com).

// Redirect user to Google's consent page to ask for permission
// for the scopes specified above.
func NewClient(ctx context.Context, config *oauth2.Config, opts ...Option) (*http.Client, error) {
	cfg := &Config{
		authCodeURLHandlerFn: PrintAuthCodeURLHandler,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	b := make([]byte, stateSize)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	state := base64.URLEncoding.EncodeToString(b)

	verifier := oauth2.GenerateVerifier()

	redirectURL, err := url.Parse(config.RedirectURL)
	if err != nil {
		return nil, err
	}

	var codeCh = make(chan string)

	server := listenAndServerRedirect(redirectURL, codeCh)

	authURL := config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	go cfg.authCodeURLHandlerFn(authURL) // run in background in case it blocks

	code := <-codeCh // block until code is read
	if err := server.Shutdown(ctx); err != nil {
		return nil, err
	}

	// Handle the exchange code to initiate a transport.
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, err
	}

	// return authenticated client with auto-refreshing token
	return config.Client(ctx, token), nil
}

func listenAndServerRedirect(redirectURL *url.URL, codeCh chan<- string) *http.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		state := query.Get(stateURLQueryKey)
		if state != string(state) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("invalid state"))
			return
		}
		codeCh <- query.Get("code")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authenticated Successfully"))
	})

	server := &http.Server{Addr: redirectURL.Host, Handler: handler}

	go func(server *http.Server) {
		slog.Debug("starting redirect server", "address", server.Addr)

		if err := server.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return // graceful shutdown
			}
			panic(fmt.Sprint("http server failure ", "error ", err))
		}
	}(server)

	return server
}

func PrintAuthCodeURLHandler(authCodeURL string) {
	fmt.Printf("Visit the URL for the auth dialog: %v\n", authCodeURL)
}

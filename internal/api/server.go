// Package api is the Prayer Deck REST API. It is described by api/openapi.yaml;
// keep the two in step when changing an endpoint.
package api

import (
	"context"
	"io"
	"net/http"

	"github.com/jesse0michael/pkg/auth"
	"github.com/jesse0michael/pkg/auth/oidc"
	"github.com/jesse0michael/pkg/http/handlers"
	"github.com/jesse0michael/pkg/http/middleware"
	"github.com/jesse0michael/prayer-deck/internal/store"
)

//go:generate go tool mockgen -destination=mock_store_test.go -package=api github.com/jesse0michael/prayer-deck/internal/store Store
//go:generate go tool mockgen -destination=mock_storage_test.go -package=api -source=server.go Storage

// Storage holds artwork objects.
type Storage interface {
	// Put uploads an object.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Delete removes objects.
	Delete(ctx context.Context, keys ...string) error
	// URL returns a URL a browser can load the object from.
	URL(ctx context.Context, key string) (string, error)
}

// LoginProvider is a sign-in method clients can exchange ID tokens through.
type LoginProvider struct {
	// Name is the provider's route segment and identity namespace, e.g. "google".
	Name string
	// ClientID is the public OAuth client ID clients need to start sign-in.
	ClientID string
	Verifier oidc.Verifier
}

// Server serves the Prayer Deck API. It satisfies pkg/http/server.Router.
type Server struct {
	store     store.Store
	storage   Storage
	auth      *auth.JWTAuth
	providers []LoginProvider
	origins   []string
	checkers  []handlers.HealthChecker
}

// Option configures a Server.
type Option func(*Server)

// WithLoginProvider enables sign-in through a provider.
func WithLoginProvider(p LoginProvider) Option {
	return func(s *Server) { s.providers = append(s.providers, p) }
}

// WithAllowedOrigins sets the browser origins allowed to call the API cross-origin.
func WithAllowedOrigins(origins ...string) Option {
	return func(s *Server) { s.origins = append(s.origins, origins...) }
}

// WithHealthCheckers adds dependencies /health verifies.
func WithHealthCheckers(checkers ...handlers.HealthChecker) Option {
	return func(s *Server) { s.checkers = append(s.checkers, checkers...) }
}

// NewServer constructs the API server.
func NewServer(st store.Store, storage Storage, jwtAuth *auth.JWTAuth, opts ...Option) *Server {
	s := &Server{store: st, storage: storage, auth: jwtAuth}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Routes builds the routing table.
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("GET /health", handlers.HandleHealth(s.checkers...))

	mux.HandleFunc("GET /v1/tokens", s.listLoginProviders)
	for _, p := range s.providers {
		mux.Handle("POST /v1/tokens/"+p.Name, handlers.HandleOIDCLogin(p.Verifier, s.auth, s.resolveIdentity))
	}
	mux.Handle("POST /v1/tokens/refresh", handlers.HandleRefreshTokens(s.auth))

	mux.Handle("GET /v1/me", s.authed(s.getMe))

	mux.Handle("GET /v1/decks", s.authed(s.listDecks))
	mux.Handle("POST /v1/decks", s.authed(s.createDeck))
	mux.Handle("PATCH /v1/decks/{deckID}", s.authed(s.renameDeck))
	mux.Handle("DELETE /v1/decks/{deckID}", s.authed(s.deleteDeck))
	mux.Handle("POST /v1/decks/{deckID}/shuffle", s.authed(s.shuffleDeck))

	mux.Handle("GET /v1/decks/{deckID}/cards", s.authed(s.listCards))
	mux.Handle("POST /v1/decks/{deckID}/cards", s.authed(s.createCard))
	mux.Handle("GET /v1/cards/{cardID}", s.authed(s.getCard))
	mux.Handle("PATCH /v1/cards/{cardID}", s.authed(s.updateCard))
	mux.Handle("DELETE /v1/cards/{cardID}", s.authed(s.deleteCard))
	mux.Handle("PUT /v1/cards/{cardID}/artwork", s.authed(s.uploadArtwork))

	mux.Handle("POST /v1/decks/{deckID}/draws", s.authed(s.drawCards))
	mux.Handle("GET /v1/decks/{deckID}/draws/current", s.authed(s.currentDraw))

	// CORS answers preflight requests before the method-matched routes see them.
	root := http.NewServeMux()
	root.Handle("/", handlers.HandleWithMiddleware(mux, logRequests, cors(s.origins)))
	return root
}

func (s *Server) authed(h http.HandlerFunc) http.Handler {
	return handlers.HandleWithMiddleware(h, middleware.Default, middleware.Auth(s.auth))
}

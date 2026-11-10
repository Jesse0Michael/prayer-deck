// Command app is the Prayer Deck API: accounts, decks, cards, artwork, and draws.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jesse0michael/pkg/auth"
	"github.com/jesse0michael/pkg/auth/oidc"
	"github.com/jesse0michael/pkg/boot"
	"github.com/jesse0michael/pkg/config"
	"github.com/jesse0michael/pkg/http/handlers"
	httpserver "github.com/jesse0michael/pkg/http/server"
	"github.com/jesse0michael/pkg/logger"
	"github.com/jesse0michael/prayer-deck/internal/api"
	"github.com/jesse0michael/prayer-deck/internal/ent"
	"github.com/jesse0michael/prayer-deck/internal/media"
	"github.com/jesse0michael/prayer-deck/internal/store"
)

// Config is loaded by boot from env vars and config files. Nested configs are
// excluded from CLI parsing, which only supports flat fields.
type Config struct {
	App      config.AppConfig      `arg:"-"`
	Logger   logger.Config         `arg:"-"`
	Server   httpserver.Config     `arg:"-"`
	Auth     auth.Config           `arg:"-"`
	Google   oidc.GoogleAuthConfig `arg:"-"`
	Postgres config.PostgresConfig `arg:"-"`
	Storage  media.Config          `arg:"-"`
	// AllowedOrigins are the web app origins allowed to call the API from a browser.
	AllowedOrigins []string `envconfig:"ALLOWED_ORIGINS"`
	// DevLogin enables signing in with just an email address. Local development only.
	DevLogin bool `envconfig:"DEV_LOGIN"`
}

func main() {
	app := boot.NewApp[Config]()
	if err := app.Run(&service{}); err != nil {
		os.Exit(1)
	}
}

// service is the boot.Runner for the API. Run builds dependencies and starts
// serving without blocking; Close shuts down gracefully.
type service struct {
	httpServer *http.Server
	entClient  *ent.Client
}

func (s *service) Run(ctx context.Context, cfg Config) error {
	db, err := config.NewPostgresClient(cfg.Postgres)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.entClient = ent.NewClient(ent.Driver(entsql.OpenDB("postgres", db.DB)))
	// Automatic migration suits the MVP; move to versioned migrations before production data exists.
	if err := s.entClient.Schema.Create(ctx); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}

	storage, err := media.NewStorage(ctx, cfg.Storage)
	if err != nil {
		return fmt.Errorf("failed to connect to storage: %w", err)
	}

	opts := []api.Option{
		api.WithAllowedOrigins(cfg.AllowedOrigins...),
		api.WithHealthCheckers(handlers.HealthChecker(storage)),
	}
	if len(cfg.Google.ClientIDs) > 0 {
		// Discovery reaches Google, so the API needs network at startup when Google sign-in is enabled.
		google, err := oidc.NewGoogleAuth(ctx, cfg.Google)
		if err != nil {
			return fmt.Errorf("failed to create google verifier: %w", err)
		}
		opts = append(opts, api.WithLoginProvider(api.LoginProvider{Name: "google", ClientID: cfg.Google.ClientIDs[0], Verifier: google}))
	}
	if cfg.DevLogin {
		slog.WarnContext(ctx, "dev login is enabled; anyone can sign in as any email")
		opts = append(opts, api.WithLoginProvider(api.LoginProvider{Name: "dev", Verifier: api.DevVerifier{}}))
	}

	jwtAuth := auth.NewJWTAuth(cfg.Auth, jwt.SigningMethodHS256)
	srv := api.NewServer(store.New(s.entClient), storage, jwtAuth, opts...)
	s.httpServer = httpserver.New(cfg.Server, srv).Server

	// Bind synchronously so a port conflict fails Run instead of a background goroutine.
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.httpServer.Addr, err)
	}
	slog.InfoContext(ctx, "api server started", "port", cfg.Server.Port)
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "http server stopped", "err", err)
		}
	}()
	return nil
}

func (s *service) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			slog.Error("failed to shut down http server", "err", err)
		}
	}
	if s.entClient != nil {
		return s.entClient.Close()
	}
	return nil
}

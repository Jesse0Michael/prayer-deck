package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/jesse0michael/pkg/auth"
	"github.com/jesse0michael/pkg/auth/oidc"
	httperrors "github.com/jesse0michael/pkg/http/errors"
	"github.com/jesse0michael/pkg/http/server"
	"github.com/jesse0michael/prayer-deck/internal/store"
)

// resolveIdentity is the OIDCResolver for every login provider.
func (s *Server) resolveIdentity(ctx context.Context, claims *oidc.Claims) ([]auth.TokenOption, error) {
	if claims.Email == "" {
		return nil, httperrors.NewError(http.StatusUnauthorized, "provider did not share an email", "")
	}

	u, err := s.store.ResolveIdentity(ctx, store.Identity{
		Provider:      claims.Provider,
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		PictureURL:    claims.Picture,
	})
	if err != nil {
		return nil, storeError(err)
	}
	return []auth.TokenOption{auth.WithSubject(u.ID.String())}, nil
}

type loginProviderView struct {
	Name     string `json:"name"`
	ClientID string `json:"client_id,omitempty"`
}

type loginProvidersResponse struct {
	Providers []loginProviderView `json:"providers"`
}

func (s *Server) listLoginProviders(w http.ResponseWriter, _ *http.Request) {
	resp := loginProvidersResponse{Providers: make([]loginProviderView, len(s.providers))}
	for i, p := range s.providers {
		resp.Providers[i] = loginProviderView{Name: p.Name, ClientID: p.ClientID}
	}
	_ = server.Encode(w, http.StatusOK, resp)
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	u, err := s.store.User(ctx, userID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusOK, toUserView(u))
}

// DevVerifier accepts an email address as its ID token. It exists so the app
// can be run locally without Google credentials and must never be enabled in
// a deployed environment.
type DevVerifier struct{}

// Verify treats rawIDToken as the email address to sign in as.
func (DevVerifier) Verify(_ context.Context, rawIDToken string) (*oidc.Claims, error) {
	email := strings.ToLower(strings.TrimSpace(rawIDToken))
	name, _, ok := strings.Cut(email, "@")
	if !ok || name == "" {
		return nil, httperrors.NewError(http.StatusUnauthorized, "dev login expects an email address", "")
	}
	return &oidc.Claims{
		Provider:      "dev",
		Subject:       email,
		Email:         email,
		EmailVerified: true,
		Name:          name,
	}, nil
}

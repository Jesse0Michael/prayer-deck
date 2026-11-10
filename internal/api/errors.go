package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jesse0michael/pkg/auth"
	httperrors "github.com/jesse0michael/pkg/http/errors"
	"github.com/jesse0michael/prayer-deck/internal/store"
)

// storeError maps store errors to HTTP errors, passing unknown errors through as 500s.
func storeError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return httperrors.NewError(http.StatusNotFound, "not found", "")
	case errors.Is(err, store.ErrInvalid):
		return httperrors.NewError(http.StatusBadRequest, "invalid input", err.Error())
	case errors.Is(err, store.ErrConflict):
		return httperrors.NewError(http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, store.ErrEmptyDeck):
		return httperrors.NewError(http.StatusUnprocessableEntity, "deck has no cards", "")
	default:
		return err
	}
}

// subject returns the authenticated user's ID.
func subject(r *http.Request) (uuid.UUID, error) {
	sub, ok := auth.Subject(r.Context())
	if !ok {
		return uuid.Nil, httperrors.NewError(http.StatusUnauthorized, "unauthenticated", "")
	}
	id, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, httperrors.NewError(http.StatusUnauthorized, "unauthenticated", "")
	}
	return id, nil
}

// pathID parses a UUID path parameter.
func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, httperrors.NewError(http.StatusNotFound, "not found", "")
	}
	return id, nil
}

package api

import (
	"log/slog"
	"net/http"
	"strings"

	httperrors "github.com/jesse0michael/pkg/http/errors"
	"github.com/jesse0michael/pkg/http/server"
)

type deckRequest struct {
	Name string `json:"name"`
}

type decksResponse struct {
	Decks []deckView `json:"decks"`
}

func (s *Server) listDecks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	decks, err := s.store.Decks(ctx, userID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	resp := decksResponse{Decks: make([]deckView, len(decks))}
	for i, d := range decks {
		resp.Decks[i] = toDeckView(d)
	}
	_ = server.Encode(w, http.StatusOK, resp)
}

func (s *Server) createDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	var req deckRequest
	if err := server.Decode(r.Body, &req); err != nil {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "invalid request body", err.Error()))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "name is required", ""))
		return
	}

	d, err := s.store.CreateDeck(ctx, userID, name)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusCreated, toDeckView(d))
}

func (s *Server) renameDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	deckID, err := pathID(r, "deckID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	var req deckRequest
	if err := server.Decode(r.Body, &req); err != nil {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "invalid request body", err.Error()))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "name is required", ""))
		return
	}

	d, err := s.store.RenameDeck(ctx, userID, deckID, name)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusOK, toDeckView(d))
}

func (s *Server) deleteDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	deckID, err := pathID(r, "deckID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	keys, err := s.store.DeleteDeck(ctx, userID, deckID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	// The deck is already gone; leftover objects are unreachable, so a
	// storage failure is logged rather than failing the request.
	if err := s.storage.Delete(ctx, keys...); err != nil {
		slog.ErrorContext(ctx, "failed to delete deck artwork", "err", err, "deck_id", deckID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) shuffleDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	deckID, err := pathID(r, "deckID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	d, err := s.store.Shuffle(ctx, userID, deckID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusOK, toDeckView(d))
}

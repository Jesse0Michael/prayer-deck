package api

import (
	"bytes"
	"io"
	"net/http"

	httperrors "github.com/jesse0michael/pkg/http/errors"
	"github.com/jesse0michael/pkg/http/server"
)

const (
	defaultDrawCount = 3
	maxDrawCount     = 10
)

type drawRequest struct {
	Count int `json:"count"`
}

func (s *Server) drawCards(w http.ResponseWriter, r *http.Request) {
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

	req := drawRequest{Count: defaultDrawCount}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<10))
	if err != nil {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "failed to read request body", err.Error()))
		return
	}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := server.Decode(bytes.NewReader(body), &req); err != nil {
			httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "invalid request body", err.Error()))
			return
		}
	}
	if req.Count < 1 || req.Count > maxDrawCount {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "count must be between 1 and 10", ""))
		return
	}

	d, err := s.store.Draw(ctx, userID, deckID, req.Count)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	view, err := s.toDrawView(ctx, d)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusCreated, view)
}

func (s *Server) currentDraw(w http.ResponseWriter, r *http.Request) {
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

	d, err := s.store.CurrentDraw(ctx, userID, deckID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	view, err := s.toDrawView(ctx, d)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusOK, view)
}

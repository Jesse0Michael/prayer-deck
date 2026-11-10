package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	httperrors "github.com/jesse0michael/pkg/http/errors"
	"github.com/jesse0michael/pkg/http/server"
	"github.com/jesse0michael/prayer-deck/internal/ent/asset"
	"github.com/jesse0michael/prayer-deck/internal/media"
	"github.com/jesse0michael/prayer-deck/internal/store"
)

// maxArtworkBytes bounds artwork uploads; generated illustrations run a few megabytes.
const maxArtworkBytes = 25 << 20

type cardRequest struct {
	Name  *string `json:"name"`
	Notes *string `json:"notes"`
}

// input validates the request. requireName is set when creating a card.
func (req cardRequest) input(requireName bool) (store.CardInput, error) {
	var in store.CardInput
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return in, httperrors.NewError(http.StatusBadRequest, "name must not be empty", "")
		}
		in.Name = &name
	} else if requireName {
		return in, httperrors.NewError(http.StatusBadRequest, "name is required", "")
	}
	if req.Notes != nil {
		notes := strings.TrimSpace(*req.Notes)
		in.Notes = &notes
	}
	return in, nil
}

type cardsResponse struct {
	Cards []cardView `json:"cards"`
}

func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
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

	cards, err := s.store.Cards(ctx, userID, deckID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	views, err := s.toCardViews(ctx, cards)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusOK, cardsResponse{Cards: views})
}

func (s *Server) createCard(w http.ResponseWriter, r *http.Request) {
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
	var req cardRequest
	if err := server.Decode(r.Body, &req); err != nil {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "invalid request body", err.Error()))
		return
	}
	in, err := req.input(true)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	c, err := s.store.CreateCard(ctx, userID, deckID, in)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	view, err := s.toCardView(ctx, c)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusCreated, view)
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	cardID, err := pathID(r, "cardID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	c, err := s.store.Card(ctx, userID, cardID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	view, err := s.toCardView(ctx, c)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusOK, view)
}

func (s *Server) updateCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	cardID, err := pathID(r, "cardID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	var req cardRequest
	if err := server.Decode(r.Body, &req); err != nil {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "invalid request body", err.Error()))
		return
	}
	in, err := req.input(false)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	c, err := s.store.UpdateCard(ctx, userID, cardID, in)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	view, err := s.toCardView(ctx, c)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusOK, view)
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	cardID, err := pathID(r, "cardID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	keys, err := s.store.DeleteCard(ctx, userID, cardID)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	// The card is already gone; leftover objects are unreachable, so a
	// storage failure is logged rather than failing the request.
	if err := s.storage.Delete(ctx, keys...); err != nil {
		slog.ErrorContext(ctx, "failed to delete card artwork", "err", err, "card_id", cardID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// uploadArtwork replaces a card's artwork with a multipart "image" upload.
func (s *Server) uploadArtwork(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	cardID, err := pathID(r, "cardID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	// Confirm ownership before accepting the upload so nothing is stored for a card the user cannot change.
	if _, err := s.store.Card(ctx, userID, cardID); err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxArtworkBytes)
	file, _, err := r.FormFile("image")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusRequestEntityTooLarge, "image is too large", fmt.Sprintf("limit is %d MB", maxArtworkBytes>>20)))
			return
		}
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "image file is required", err.Error()))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusBadRequest, "failed to read image", err.Error()))
		return
	}

	renditions, err := media.Renditions(data)
	if err != nil {
		if errors.Is(err, media.ErrUnsupportedImage) {
			httperrors.WriteError(ctx, w, httperrors.NewError(http.StatusUnsupportedMediaType, "unsupported image", "upload a PNG, JPEG, WebP, or GIF"))
			return
		}
		httperrors.WriteError(ctx, w, err)
		return
	}

	assets := make([]store.AssetInput, 0, len(renditions))
	var uploaded []string
	for _, rd := range renditions {
		id := uuid.New()
		key := fmt.Sprintf("cards/%s/%s-%s.%s", cardID, id, rd.Name, rd.Extension)
		if err := s.storage.Put(ctx, key, bytes.NewReader(rd.Data), int64(len(rd.Data)), rd.ContentType); err != nil {
			s.discard(r, uploaded)
			httperrors.WriteError(ctx, w, err)
			return
		}
		uploaded = append(uploaded, key)
		assets = append(assets, store.AssetInput{
			ID:          id,
			Rendition:   asset.Rendition(rd.Name),
			Key:         key,
			ContentType: rd.ContentType,
			Size:        int64(len(rd.Data)),
			Width:       rd.Width,
			Height:      rd.Height,
		})
	}

	c, replaced, err := s.store.ReplaceArtwork(ctx, userID, cardID, assets)
	if err != nil {
		s.discard(r, uploaded)
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	s.discard(r, replaced)

	view, err := s.toCardView(ctx, c)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	_ = server.Encode(w, http.StatusOK, view)
}

// discard removes objects no record points to, logging rather than failing
// since the request outcome is already decided.
func (s *Server) discard(r *http.Request, keys []string) {
	if len(keys) == 0 {
		return
	}
	if err := s.storage.Delete(r.Context(), keys...); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete unreferenced artwork", "err", err, "keys", keys)
	}
}

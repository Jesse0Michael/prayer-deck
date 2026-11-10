package api

import (
	"net/http"
	"strconv"
	"strings"

	httperrors "github.com/jesse0michael/pkg/http/errors"
	"github.com/jesse0michael/pkg/http/server"
)

const (
	defaultNotesLimit = 6
	maxNotesLimit     = 50
	maxNoteLength     = 2000
)

type noteRequest struct {
	Text string `json:"text"`
}

// validText trims the note body and checks its length.
func (req noteRequest) validText() (string, error) {
	text := strings.TrimSpace(req.Text)
	switch {
	case text == "":
		return "", httperrors.NewError(http.StatusBadRequest, "text must not be empty", "")
	case len([]rune(text)) > maxNoteLength:
		return "", httperrors.NewError(http.StatusBadRequest, "text is too long", "limit is 2000 characters")
	}
	return text, nil
}

// queryInt reads an optional integer query parameter, bounded to [min, max].
func queryInt(r *http.Request, name string, fallback, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return 0, httperrors.NewError(http.StatusBadRequest, "invalid "+name, "must be between "+strconv.Itoa(min)+" and "+strconv.Itoa(max))
	}
	return v, nil
}

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
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
	limit, err := queryInt(r, "limit", defaultNotesLimit, 1, maxNotesLimit)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	offset, err := queryInt(r, "offset", 0, 0, 1<<30)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	notes, total, err := s.store.Notes(ctx, userID, cardID, limit, offset)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusOK, toNotesView(notes, total))
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
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
	text, err := decodeNote(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	n, err := s.store.CreateNote(ctx, userID, cardID, text)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusCreated, toNoteView(n))
}

func (s *Server) updateNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	noteID, err := pathID(r, "noteID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	text, err := decodeNote(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	n, err := s.store.UpdateNote(ctx, userID, noteID, text)
	if err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	_ = server.Encode(w, http.StatusOK, toNoteView(n))
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := subject(r)
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}
	noteID, err := pathID(r, "noteID")
	if err != nil {
		httperrors.WriteError(ctx, w, err)
		return
	}

	if err := s.store.DeleteNote(ctx, userID, noteID); err != nil {
		httperrors.WriteError(ctx, w, storeError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeNote(r *http.Request) (string, error) {
	var req noteRequest
	if err := server.Decode(r.Body, &req); err != nil {
		return "", httperrors.NewError(http.StatusBadRequest, "invalid request body", err.Error())
	}
	return req.validText()
}

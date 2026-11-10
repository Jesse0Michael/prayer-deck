package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jesse0michael/prayer-deck/internal/ent"
	"github.com/jesse0michael/prayer-deck/internal/ent/asset"
	"github.com/jesse0michael/prayer-deck/internal/store"
)

type userView struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name,omitempty"`
	PictureURL string    `json:"picture_url,omitempty"`
}

func toUserView(u *ent.User) userView {
	return userView{ID: u.ID, Email: u.Email, Name: u.Name, PictureURL: u.PictureURL}
}

type deckView struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CardCount int       `json:"card_count"`
	Remaining int       `json:"remaining"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toDeckView(d *store.Deck) deckView {
	return deckView{
		ID:        d.ID,
		Name:      d.Name,
		CardCount: d.CardCount,
		Remaining: d.Remaining,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// mediaView holds time-limited URLs to each rendition of a media item. Width
// and height are the display rendition's.
type mediaView struct {
	ID           uuid.UUID `json:"id"`
	Kind         string    `json:"kind"`
	DisplayURL   string    `json:"display_url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	OriginalURL  string    `json:"original_url"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	CreatedAt    time.Time `json:"created_at"`
}

type cardView struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Notes     string      `json:"notes"`
	Media     []mediaView `json:"media"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

func (s *Server) toCardView(ctx context.Context, c *ent.Card) (cardView, error) {
	view := cardView{
		ID:        c.ID,
		Name:      c.Name,
		Notes:     c.Notes,
		Media:     make([]mediaView, len(c.Edges.Media)),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	for i, m := range c.Edges.Media {
		mv, err := s.toMediaView(ctx, m)
		if err != nil {
			return cardView{}, err
		}
		view.Media[i] = mv
	}
	return view, nil
}

func (s *Server) toMediaView(ctx context.Context, m *ent.Media) (mediaView, error) {
	view := mediaView{ID: m.ID, Kind: m.Kind.String(), CreatedAt: m.CreatedAt}
	for _, a := range m.Edges.Assets {
		url, err := s.storage.URL(ctx, a.Key)
		if err != nil {
			return mediaView{}, err
		}
		switch a.Rendition {
		case asset.RenditionDisplay:
			view.DisplayURL = url
			view.Width = a.Width
			view.Height = a.Height
		case asset.RenditionThumbnail:
			view.ThumbnailURL = url
		case asset.RenditionOriginal:
			view.OriginalURL = url
		}
	}
	return view, nil
}

func (s *Server) toCardViews(ctx context.Context, cards []*ent.Card) ([]cardView, error) {
	views := make([]cardView, len(cards))
	for i, c := range cards {
		v, err := s.toCardView(ctx, c)
		if err != nil {
			return nil, err
		}
		views[i] = v
	}
	return views, nil
}

// drawnCardView is a dealt card with the media chosen as its face. Face is nil
// when the card has no media or the chosen media has since been deleted.
type drawnCardView struct {
	cardView
	Face *mediaView `json:"face"`
}

type drawView struct {
	ID        uuid.UUID       `json:"id"`
	Cards     []drawnCardView `json:"cards"`
	Discard   []cardView      `json:"discard"`
	Remaining int             `json:"remaining"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Server) toDrawView(ctx context.Context, d *store.Draw) (drawView, error) {
	cards := make([]drawnCardView, len(d.Cards))
	for i, c := range d.Cards {
		view, err := s.toCardView(ctx, c)
		if err != nil {
			return drawView{}, err
		}
		cards[i] = drawnCardView{cardView: view}
		faceID, ok := d.Faces[c.ID]
		if !ok {
			continue
		}
		for _, m := range view.Media {
			if m.ID == faceID {
				cards[i].Face = &m
				break
			}
		}
	}
	discard, err := s.toCardViews(ctx, d.Discard)
	if err != nil {
		return drawView{}, err
	}
	return drawView{ID: d.ID, Cards: cards, Discard: discard, Remaining: d.Remaining, CreatedAt: d.CreatedAt}, nil
}

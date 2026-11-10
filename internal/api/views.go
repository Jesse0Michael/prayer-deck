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

type artworkView struct {
	DisplayURL   string `json:"display_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	OriginalURL  string `json:"original_url"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type cardView struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Notes     string       `json:"notes"`
	Artwork   *artworkView `json:"artwork"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (s *Server) toCardView(ctx context.Context, c *ent.Card) (cardView, error) {
	view := cardView{
		ID:        c.ID,
		Name:      c.Name,
		Notes:     c.Notes,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if len(c.Edges.Assets) == 0 {
		return view, nil
	}

	artwork := &artworkView{}
	for _, a := range c.Edges.Assets {
		url, err := s.storage.URL(ctx, a.Key)
		if err != nil {
			return cardView{}, err
		}
		switch a.Rendition {
		case asset.RenditionDisplay:
			artwork.DisplayURL = url
			artwork.Width = a.Width
			artwork.Height = a.Height
		case asset.RenditionThumbnail:
			artwork.ThumbnailURL = url
		case asset.RenditionOriginal:
			artwork.OriginalURL = url
		}
	}
	view.Artwork = artwork
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

type drawView struct {
	ID        uuid.UUID  `json:"id"`
	Cards     []cardView `json:"cards"`
	Remaining int        `json:"remaining"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Server) toDrawView(ctx context.Context, d *store.Draw) (drawView, error) {
	cards, err := s.toCardViews(ctx, d.Cards)
	if err != nil {
		return drawView{}, err
	}
	return drawView{ID: d.ID, Cards: cards, Remaining: d.Remaining, CreatedAt: d.CreatedAt}, nil
}

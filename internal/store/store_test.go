package store

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jesse0michael/prayer-deck/internal/ent"
	"github.com/jesse0michael/prayer-deck/internal/ent/asset"
	"github.com/jesse0michael/prayer-deck/internal/ent/card"
	"github.com/jesse0michael/prayer-deck/internal/ent/deck"
	"github.com/jesse0michael/prayer-deck/internal/ent/enttest"
	"github.com/jesse0michael/prayer-deck/internal/ent/media"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

var (
	user1  = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	user2  = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	deck1  = uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	deck2  = uuid.MustParse("00000000-0000-0000-0000-0000000000d2")
	deck3  = uuid.MustParse("00000000-0000-0000-0000-0000000000d3")
	card1  = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	card2  = uuid.MustParse("00000000-0000-0000-0000-0000000000c2")
	card3  = uuid.MustParse("00000000-0000-0000-0000-0000000000c3")
	card4  = uuid.MustParse("00000000-0000-0000-0000-0000000000c4")
	media1 = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	media2 = uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	asset1 = uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	asset2 = uuid.MustParse("00000000-0000-0000-0000-0000000000e2")
	// t0 is the synctest bubble's starting time, which every fixture timestamp derives from.
	t0 = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
)

var entOpts = cmp.Options{
	cmpopts.IgnoreUnexported(ent.User{}, ent.UserEdges{}, ent.Deck{}, ent.DeckEdges{}, ent.Card{}, ent.CardEdges{}, ent.Media{}, ent.MediaEdges{}, ent.Asset{}, ent.AssetEdges{}, ent.Draw{}, ent.DrawEdges{}),
	cmpopts.EquateEmpty(),
}

// newTestStore opens an in-memory sqlite store seeded with:
//   - user1 owning deck1 "Family" (card1 Anna with one image, card2 Ben, card3 Cara) and deck2 "Friends" (empty)
//   - user2 owning deck3 "Other" (card4 Dan)
//
// It must be called inside a synctest bubble so timestamps are deterministic.
func newTestStore(t *testing.T) (*entStore, *ent.Client) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	client := enttest.Open(t, "sqlite3", dsn)
	t.Cleanup(func() { _ = client.Close() })
	ctx := t.Context()

	client.User.Create().SetID(user1).SetEmail("test-user-1@example.com").SetName("test-user-1").ExecX(ctx)
	client.User.Create().SetID(user2).SetEmail("test-user-2@example.com").SetName("test-user-2").ExecX(ctx)
	client.Deck.Create().SetID(deck1).SetName("Family").SetOwnerID(user1).SetShuffledAt(t0).ExecX(ctx)
	client.Deck.Create().SetID(deck2).SetName("Friends").SetOwnerID(user1).SetShuffledAt(t0).SetCreatedAt(t0.Add(time.Second)).ExecX(ctx)
	client.Deck.Create().SetID(deck3).SetName("Other").SetOwnerID(user2).SetShuffledAt(t0).ExecX(ctx)
	client.Card.Create().SetID(card1).SetName("Anna").SetDeckID(deck1).ExecX(ctx)
	client.Card.Create().SetID(card2).SetName("Ben").SetDeckID(deck1).ExecX(ctx)
	client.Card.Create().SetID(card3).SetName("Cara").SetNotes("loves gardening").SetDeckID(deck1).ExecX(ctx)
	client.Card.Create().SetID(card4).SetName("Dan").SetDeckID(deck3).ExecX(ctx)
	client.Media.Create().SetID(media1).SetCardID(card1).SetKind(media.KindImage).ExecX(ctx)
	client.Asset.Create().SetID(asset1).SetMediaID(media1).SetRendition(asset.RenditionDisplay).SetKey("cards/c1/display.jpg").
		SetContentType("image/jpeg").SetSize(100).SetWidth(10).SetHeight(15).ExecX(ctx)

	return &entStore{client: client, shuffle: func(int, func(i, j int)) {}, pick: func(n int) int { return n - 1 }, now: time.Now}, client
}

func testMedia() *ent.Media {
	return &ent.Media{ID: media1, Kind: media.KindImage, CreatedAt: t0, Edges: ent.MediaEdges{Assets: []*ent.Asset{
		{ID: asset1, Rendition: asset.RenditionDisplay, Key: "cards/c1/display.jpg", ContentType: "image/jpeg", Size: 100, Width: 10, Height: 15, CreatedAt: t0},
	}}}
}

func testCard(id uuid.UUID) *ent.Card {
	switch id {
	case card1:
		return &ent.Card{ID: card1, Name: "Anna", CreatedAt: t0, UpdatedAt: t0, Edges: ent.CardEdges{Media: []*ent.Media{testMedia()}}}
	case card2:
		return &ent.Card{ID: card2, Name: "Ben", CreatedAt: t0, UpdatedAt: t0}
	case card3:
		return &ent.Card{ID: card3, Name: "Cara", Notes: "loves gardening", CreatedAt: t0, UpdatedAt: t0}
	default:
		return &ent.Card{ID: card4, Name: "Dan", CreatedAt: t0, UpdatedAt: t0}
	}
}

// drawnAt marks a fixture card as drawn at a time; drawing also bumps updated_at.
func drawnAt(c *ent.Card, at time.Time) *ent.Card {
	c.LastDrawnAt = &at
	c.UpdatedAt = at
	return c
}

func cardIDs(cards []*ent.Card) []uuid.UUID {
	var ids []uuid.UUID
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func ptr[T any](v T) *T {
	return &v
}

func TestStore_ResolveIdentity(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, client *ent.Client)
		identity      Identity
		expected      *ent.User
		expectedDecks []string
		wantErr       error
	}{
		{
			name:          "new identity creates an account with a starter deck",
			setup:         func(*testing.T, *ent.Client) {},
			identity:      Identity{Provider: "google", Subject: "test-subject", Email: "test-new@example.com", EmailVerified: true, Name: "test-new"},
			expected:      &ent.User{ID: uuid.MustParse("52fdfc07-2182-454f-963f-5f0f9a621d72"), Email: "test-new@example.com", Name: "test-new", CreatedAt: t0, UpdatedAt: t0},
			expectedDecks: []string{DefaultDeckName},
		},
		{
			name: "linked identity returns its account",
			setup: func(t *testing.T, client *ent.Client) {
				client.Identity.Create().SetProvider("google").SetSubject("test-subject").SetUserID(user1).ExecX(t.Context())
			},
			identity:      Identity{Provider: "google", Subject: "test-subject", Email: "changed@example.com", EmailVerified: true},
			expected:      &ent.User{ID: user1, Email: "test-user-1@example.com", Name: "test-user-1", CreatedAt: t0, UpdatedAt: t0},
			expectedDecks: []string{"Family", "Friends"},
		},
		{
			name:          "new provider with a verified email joins the existing account",
			setup:         func(*testing.T, *ent.Client) {},
			identity:      Identity{Provider: "apple", Subject: "test-subject", Email: "test-user-1@example.com", EmailVerified: true},
			expected:      &ent.User{ID: user1, Email: "test-user-1@example.com", Name: "test-user-1", CreatedAt: t0, UpdatedAt: t0},
			expectedDecks: []string{"Family", "Friends"},
		},
		{
			name:     "unverified email cannot join an existing account",
			setup:    func(*testing.T, *ent.Client) {},
			identity: Identity{Provider: "apple", Subject: "test-subject", Email: "test-user-1@example.com"},
			wantErr:  ErrConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				uuid.SetRand(rand.New(rand.NewSource(1)))
				defer uuid.SetRand(nil)
				s, client := newTestStore(t)
				tt.setup(t, client)

				u, err := s.ResolveIdentity(t.Context(), tt.identity)

				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, cmp.Diff(tt.expected, u, entOpts))
				if tt.expected != nil {
					decks, err := s.Decks(t.Context(), tt.expected.ID)
					require.NoError(t, err)
					names := make([]string, len(decks))
					for i, d := range decks {
						names[i] = d.Name
					}
					require.Equal(t, tt.expectedDecks, names)
				}
			})
		})
	}
}

func TestStore_Decks(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, client *ent.Client)
		userID   uuid.UUID
		expected []*Deck
	}{
		{
			name: "counts cards and the remaining pass",
			setup: func(t *testing.T, client *ent.Client) {
				client.Card.UpdateOneID(card2).SetLastDrawnAt(t0.Add(time.Hour)).ExecX(t.Context())
			},
			userID: user1,
			expected: []*Deck{
				{Deck: &ent.Deck{ID: deck1, Name: "Family", ShuffledAt: t0, CreatedAt: t0, UpdatedAt: t0}, CardCount: 3, Remaining: 2},
				{Deck: &ent.Deck{ID: deck2, Name: "Friends", ShuffledAt: t0, CreatedAt: t0.Add(time.Second), UpdatedAt: t0}, CardCount: 0, Remaining: 0},
			},
		},
		{
			name:     "user without decks",
			setup:    func(*testing.T, *ent.Client) {},
			userID:   uuid.MustParse("00000000-0000-0000-0000-0000000000ff"),
			expected: []*Deck{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, client := newTestStore(t)
				tt.setup(t, client)

				decks, err := s.Decks(t.Context(), tt.userID)

				require.NoError(t, err)
				require.Empty(t, cmp.Diff(tt.expected, decks, entOpts))
			})
		})
	}
}

func TestStore_DeleteDeck(t *testing.T) {
	tests := []struct {
		name          string
		userID        uuid.UUID
		deckID        uuid.UUID
		expectedKeys  []string
		expectedDecks int
		wantErr       error
	}{
		{
			name:          "deletes the deck and returns its media keys",
			userID:        user1,
			deckID:        deck1,
			expectedKeys:  []string{"cards/c1/display.jpg"},
			expectedDecks: 1,
		},
		{
			name:          "keeps the last deck",
			userID:        user2,
			deckID:        deck3,
			expectedDecks: 1,
			wantErr:       ErrConflict,
		},
		{
			name:          "another user's deck is not found",
			userID:        user2,
			deckID:        deck1,
			expectedDecks: 1,
			wantErr:       ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _ := newTestStore(t)

				keys, err := s.DeleteDeck(t.Context(), tt.userID, tt.deckID)

				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, tt.expectedKeys, keys)
				decks, err := s.Decks(t.Context(), tt.userID)
				require.NoError(t, err)
				require.Len(t, decks, tt.expectedDecks)
			})
		})
	}
}

func TestStore_CreateCard(t *testing.T) {
	tests := []struct {
		name     string
		userID   uuid.UUID
		deckID   uuid.UUID
		input    CardInput
		expected *ent.Card
		wantErr  error
	}{
		{
			name:     "creates a card",
			userID:   user1,
			deckID:   deck2,
			input:    CardInput{Name: ptr("Eve"), Notes: ptr("two dogs")},
			expected: &ent.Card{ID: uuid.MustParse("52fdfc07-2182-454f-963f-5f0f9a621d72"), Name: "Eve", Notes: "two dogs", CreatedAt: t0, UpdatedAt: t0},
		},
		{
			name:    "empty name is invalid",
			userID:  user1,
			deckID:  deck2,
			input:   CardInput{Name: ptr("")},
			wantErr: ErrInvalid,
		},
		{
			name:    "another user's deck is not found",
			userID:  user2,
			deckID:  deck2,
			input:   CardInput{Name: ptr("Eve")},
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				uuid.SetRand(rand.New(rand.NewSource(1)))
				defer uuid.SetRand(nil)
				s, _ := newTestStore(t)

				c, err := s.CreateCard(t.Context(), tt.userID, tt.deckID, tt.input)

				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, cmp.Diff(tt.expected, c, entOpts))
			})
		})
	}
}

func TestStore_UpdateCard(t *testing.T) {
	tests := []struct {
		name     string
		userID   uuid.UUID
		cardID   uuid.UUID
		input    CardInput
		expected *ent.Card
		wantErr  error
	}{
		{
			name:   "changes only the given fields",
			userID: user1,
			cardID: card1,
			input:  CardInput{Notes: ptr("plays cello")},
			expected: func() *ent.Card {
				c := testCard(card1)
				c.Notes = "plays cello"
				return c
			}(),
		},
		{
			name:    "another user's card is not found",
			userID:  user2,
			cardID:  card1,
			input:   CardInput{Name: ptr("Mallory")},
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _ := newTestStore(t)

				c, err := s.UpdateCard(t.Context(), tt.userID, tt.cardID, tt.input)

				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, cmp.Diff(tt.expected, c, entOpts))
			})
		})
	}
}

func TestStore_DeleteCard(t *testing.T) {
	tests := []struct {
		name         string
		userID       uuid.UUID
		cardID       uuid.UUID
		expectedKeys []string
		wantErr      error
	}{
		{
			name:         "deletes the card and returns its media keys",
			userID:       user1,
			cardID:       card1,
			expectedKeys: []string{"cards/c1/display.jpg"},
		},
		{
			name:         "card without media",
			userID:       user1,
			cardID:       card2,
			expectedKeys: []string{},
		},
		{
			name:    "another user's card is not found",
			userID:  user2,
			cardID:  card1,
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _ := newTestStore(t)

				keys, err := s.DeleteCard(t.Context(), tt.userID, tt.cardID)

				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, tt.expectedKeys, keys)
			})
		})
	}
}

func TestStore_AddMedia(t *testing.T) {
	addTime := t0.Add(time.Hour)
	in := MediaInput{
		ID:   media2,
		Kind: media.KindImage,
		Assets: []AssetInput{
			{ID: asset2, Rendition: asset.RenditionThumbnail, Key: "cards/c1/b2/thumbnail.jpg", ContentType: "image/jpeg", Size: 50, Width: 4, Height: 6},
		},
	}
	tests := []struct {
		name     string
		userID   uuid.UUID
		cardID   uuid.UUID
		expected *ent.Card
		wantErr  error
	}{
		{
			name:   "adds the newest media first",
			userID: user1,
			cardID: card1,
			expected: &ent.Card{ID: card1, Name: "Anna", CreatedAt: t0, UpdatedAt: addTime, Edges: ent.CardEdges{Media: []*ent.Media{
				{ID: media2, Kind: media.KindImage, CreatedAt: addTime, Edges: ent.MediaEdges{Assets: []*ent.Asset{
					{ID: asset2, Rendition: asset.RenditionThumbnail, Key: "cards/c1/b2/thumbnail.jpg", ContentType: "image/jpeg", Size: 50, Width: 4, Height: 6, CreatedAt: addTime},
				}}},
				testMedia(),
			}}},
		},
		{
			name:   "first media on a card",
			userID: user1,
			cardID: card2,
			expected: &ent.Card{ID: card2, Name: "Ben", CreatedAt: t0, UpdatedAt: addTime, Edges: ent.CardEdges{Media: []*ent.Media{
				{ID: media2, Kind: media.KindImage, CreatedAt: addTime, Edges: ent.MediaEdges{Assets: []*ent.Asset{
					{ID: asset2, Rendition: asset.RenditionThumbnail, Key: "cards/c1/b2/thumbnail.jpg", ContentType: "image/jpeg", Size: 50, Width: 4, Height: 6, CreatedAt: addTime},
				}}},
			}}},
		},
		{
			name:    "another user's card is not found",
			userID:  user2,
			cardID:  card1,
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _ := newTestStore(t)
				time.Sleep(addTime.Sub(t0))

				c, err := s.AddMedia(t.Context(), tt.userID, tt.cardID, in)

				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, cmp.Diff(tt.expected, c, entOpts))
			})
		})
	}
}

func TestStore_DeleteMedia(t *testing.T) {
	tests := []struct {
		name          string
		userID        uuid.UUID
		cardID        uuid.UUID
		mediaID       uuid.UUID
		expectedKeys  []string
		expectedMedia int
		wantErr       error
	}{
		{
			name:          "deletes the media and returns its keys",
			userID:        user1,
			cardID:        card1,
			mediaID:       media1,
			expectedKeys:  []string{"cards/c1/display.jpg"},
			expectedMedia: 0,
		},
		{
			name:          "media on another card is not found",
			userID:        user1,
			cardID:        card2,
			mediaID:       media1,
			expectedMedia: 1,
			wantErr:       ErrNotFound,
		},
		{
			name:          "another user's media is not found",
			userID:        user2,
			cardID:        card1,
			mediaID:       media1,
			expectedMedia: 1,
			wantErr:       ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, client := newTestStore(t)

				keys, err := s.DeleteMedia(t.Context(), tt.userID, tt.cardID, tt.mediaID)

				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, tt.expectedKeys, keys)
				require.Equal(t, tt.expectedMedia, client.Media.Query().CountX(t.Context()))
				require.Equal(t, tt.expectedMedia, client.Asset.Query().CountX(t.Context()))
			})
		})
	}
}

func TestStore_Draw(t *testing.T) {
	// drawTime is when each test draws; fixtures drawn "earlier this pass" use t0+1m.
	drawTime := t0.Add(time.Hour)
	tests := []struct {
		name               string
		setup              func(t *testing.T, client *ent.Client)
		userID             uuid.UUID
		deckID             uuid.UUID
		count              int
		reshuffle          bool
		expectedCards      []*ent.Card
		expectedFaces      map[uuid.UUID]uuid.UUID
		expectedDiscard    []*ent.Card
		expectedRemaining  int
		expectedShuffledAt time.Time
		wantErr            error
	}{
		{
			name:               "draws from the start of a pass",
			setup:              func(*testing.T, *ent.Client) {},
			userID:             user1,
			deckID:             deck1,
			count:              2,
			expectedCards:      []*ent.Card{drawnAt(testCard(card1), drawTime), drawnAt(testCard(card2), drawTime)},
			expectedFaces:      map[uuid.UUID]uuid.UUID{card1: media1},
			expectedRemaining:  1,
			expectedShuffledAt: t0,
		},
		{
			name: "picks a face from a card's media",
			setup: func(t *testing.T, client *ent.Client) {
				client.Media.Create().SetID(media2).SetCardID(card1).SetKind(media.KindImage).SetCreatedAt(t0.Add(time.Minute)).ExecX(t.Context())
			},
			userID: user1,
			deckID: deck1,
			count:  1,
			expectedCards: []*ent.Card{{ID: card1, Name: "Anna", LastDrawnAt: &drawTime, CreatedAt: t0, UpdatedAt: drawTime, Edges: ent.CardEdges{Media: []*ent.Media{
				{ID: media2, Kind: media.KindImage, CreatedAt: t0.Add(time.Minute)},
				testMedia(),
			}}}},
			expectedFaces:      map[uuid.UUID]uuid.UUID{card1: media1},
			expectedRemaining:  2,
			expectedShuffledAt: t0,
		},
		{
			name: "skips cards already drawn this pass and discards them",
			setup: func(t *testing.T, client *ent.Client) {
				client.Card.UpdateOneID(card1).SetLastDrawnAt(t0.Add(time.Minute)).SetUpdatedAt(t0.Add(time.Minute)).ExecX(t.Context())
			},
			userID:             user1,
			deckID:             deck1,
			count:              2,
			expectedCards:      []*ent.Card{drawnAt(testCard(card2), drawTime), drawnAt(testCard(card3), drawTime)},
			expectedDiscard:    []*ent.Card{drawnAt(testCard(card1), t0.Add(time.Minute))},
			expectedRemaining:  0,
			expectedShuffledAt: t0,
		},
		{
			name: "deals a short hand when the pass is nearly finished",
			setup: func(t *testing.T, client *ent.Client) {
				client.Card.Update().SetLastDrawnAt(t0.Add(time.Minute)).SetUpdatedAt(t0.Add(time.Minute)).Where(card.IDIn(card1, card2)).ExecX(t.Context())
			},
			userID:             user1,
			deckID:             deck1,
			count:              2,
			expectedCards:      []*ent.Card{drawnAt(testCard(card3), drawTime)},
			expectedDiscard:    []*ent.Card{drawnAt(testCard(card1), t0.Add(time.Minute)), drawnAt(testCard(card2), t0.Add(time.Minute))},
			expectedRemaining:  0,
			expectedShuffledAt: t0,
		},
		{
			name: "finished pass",
			setup: func(t *testing.T, client *ent.Client) {
				client.Card.Update().SetLastDrawnAt(t0.Add(time.Minute)).Where(card.HasDeckWith(deck.ID(deck1))).ExecX(t.Context())
			},
			userID:  user1,
			deckID:  deck1,
			count:   3,
			wantErr: ErrPassFinished,
		},
		{
			name: "shuffle starts a fresh pass before dealing",
			setup: func(t *testing.T, client *ent.Client) {
				client.Card.Update().SetLastDrawnAt(t0.Add(time.Minute)).Where(card.HasDeckWith(deck.ID(deck1))).ExecX(t.Context())
			},
			userID:             user1,
			deckID:             deck1,
			count:              2,
			reshuffle:          true,
			expectedCards:      []*ent.Card{drawnAt(testCard(card1), drawTime), drawnAt(testCard(card2), drawTime)},
			expectedFaces:      map[uuid.UUID]uuid.UUID{card1: media1},
			expectedRemaining:  1,
			expectedShuffledAt: drawTime,
		},
		{
			name:               "count beyond the deck deals every card",
			setup:              func(*testing.T, *ent.Client) {},
			userID:             user1,
			deckID:             deck1,
			count:              5,
			expectedCards:      []*ent.Card{drawnAt(testCard(card1), drawTime), drawnAt(testCard(card2), drawTime), drawnAt(testCard(card3), drawTime)},
			expectedFaces:      map[uuid.UUID]uuid.UUID{card1: media1},
			expectedRemaining:  0,
			expectedShuffledAt: t0,
		},
		{
			name:    "empty deck",
			setup:   func(*testing.T, *ent.Client) {},
			userID:  user1,
			deckID:  deck2,
			count:   3,
			wantErr: ErrEmptyDeck,
		},
		{
			name:    "another user's deck is not found",
			setup:   func(*testing.T, *ent.Client) {},
			userID:  user2,
			deckID:  deck1,
			count:   3,
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, client := newTestStore(t)
				tt.setup(t, client)
				time.Sleep(drawTime.Sub(t0))

				d, err := s.Draw(t.Context(), tt.userID, tt.deckID, tt.count, tt.reshuffle)

				require.ErrorIs(t, err, tt.wantErr)
				if tt.wantErr != nil {
					return
				}
				require.Empty(t, cmp.Diff(tt.expectedCards, d.Cards, entOpts))
				require.Empty(t, cmp.Diff(tt.expectedFaces, d.Faces, entOpts))
				require.Empty(t, cmp.Diff(tt.expectedDiscard, d.Discard, entOpts))
				require.Equal(t, tt.expectedRemaining, d.Remaining)
				ids := make([]uuid.UUID, len(tt.expectedCards))
				for i, c := range tt.expectedCards {
					ids[i] = c.ID
				}
				require.Equal(t, ids, d.CardIds)
				deck := client.Deck.GetX(t.Context(), tt.deckID)
				require.Equal(t, tt.expectedShuffledAt, deck.ShuffledAt)
				require.Equal(t, 1, client.Draw.Query().CountX(t.Context()))
			})
		})
	}
}

func TestStore_CurrentDraw(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T, s *entStore, client *ent.Client)
		userID          uuid.UUID
		expectedCards   []uuid.UUID
		expectedDiscard []uuid.UUID
		wantErr         error
	}{
		{
			name: "returns the latest draw in dealt order with the earlier hand discarded",
			setup: func(t *testing.T, s *entStore, _ *ent.Client) {
				_, err := s.Draw(t.Context(), user1, deck1, 1, false)
				require.NoError(t, err)
				time.Sleep(time.Minute)
				s.shuffle = func(n int, swap func(i, j int)) { swap(0, n-1) }
				_, err = s.Draw(t.Context(), user1, deck1, 2, false)
				require.NoError(t, err)
			},
			userID:          user1,
			expectedCards:   []uuid.UUID{card3, card2},
			expectedDiscard: []uuid.UUID{card1},
		},
		{
			name: "shuffling empties the discard",
			setup: func(t *testing.T, s *entStore, _ *ent.Client) {
				_, err := s.Draw(t.Context(), user1, deck1, 2, false)
				require.NoError(t, err)
				time.Sleep(time.Minute)
				_, err = s.Draw(t.Context(), user1, deck1, 1, true)
				require.NoError(t, err)
			},
			userID:        user1,
			expectedCards: []uuid.UUID{card1},
		},
		{
			name: "skips cards deleted since the draw",
			setup: func(t *testing.T, s *entStore, client *ent.Client) {
				_, err := s.Draw(t.Context(), user1, deck1, 3, false)
				require.NoError(t, err)
				client.Card.DeleteOneID(card2).ExecX(t.Context())
			},
			userID:        user1,
			expectedCards: []uuid.UUID{card1, card3},
		},
		{
			name:    "no draw yet",
			setup:   func(*testing.T, *entStore, *ent.Client) {},
			userID:  user1,
			wantErr: ErrNotFound,
		},
		{
			name: "another user's draw is not found",
			setup: func(t *testing.T, s *entStore, _ *ent.Client) {
				_, err := s.Draw(t.Context(), user1, deck1, 3, false)
				require.NoError(t, err)
			},
			userID:  user2,
			wantErr: ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, client := newTestStore(t)
				tt.setup(t, s, client)

				d, err := s.CurrentDraw(t.Context(), tt.userID, deck1)

				require.ErrorIs(t, err, tt.wantErr)
				if tt.wantErr != nil {
					return
				}
				require.Equal(t, tt.expectedCards, cardIDs(d.Cards))
				require.Equal(t, tt.expectedDiscard, cardIDs(d.Discard))
			})
		})
	}
}

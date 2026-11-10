// Package store is the persistence seam for Prayer Deck: accounts, decks,
// cards, media, and draws, backed by ent. Every read and write is
// scoped to the requesting user, so another user's records look like they
// do not exist.
package store

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jesse0michael/prayer-deck/internal/ent"
	"github.com/jesse0michael/prayer-deck/internal/ent/asset"
	"github.com/jesse0michael/prayer-deck/internal/ent/card"
	"github.com/jesse0michael/prayer-deck/internal/ent/deck"
	"github.com/jesse0michael/prayer-deck/internal/ent/draw"
	"github.com/jesse0michael/prayer-deck/internal/ent/identity"
	"github.com/jesse0michael/prayer-deck/internal/ent/media"
	"github.com/jesse0michael/prayer-deck/internal/ent/predicate"
	"github.com/jesse0michael/prayer-deck/internal/ent/user"
)

var (
	// ErrNotFound is returned when a record does not exist or belongs to another user.
	ErrNotFound = errors.New("not found")
	// ErrInvalid is returned when input fails schema validation.
	ErrInvalid = errors.New("invalid")
	// ErrConflict is returned when a change would leave the account in an unusable state.
	ErrConflict = errors.New("conflict")
	// ErrEmptyDeck is returned when drawing from a deck with no cards.
	ErrEmptyDeck = errors.New("deck has no cards")
	// ErrPassFinished is returned when every card in the pass has been drawn and the deck needs a shuffle.
	ErrPassFinished = errors.New("every card in the pass has been drawn")
)

// DefaultDeckName names the deck every new account starts with.
const DefaultDeckName = "My Deck"

// Identity is a verified sign-in from an external provider.
type Identity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	PictureURL    string
}

// Deck is a deck with the counts the deck picker shows.
type Deck struct {
	*ent.Deck
	CardCount int
	// Remaining is how many cards are left to draw in the pass.
	Remaining int
}

// CardInput carries card fields to create or change. Nil fields are left unchanged on update.
type CardInput struct {
	Name  *string
	Notes *string
}

// MediaInput describes a new media item and its stored renditions.
type MediaInput struct {
	ID     uuid.UUID
	Kind   media.Kind
	Assets []AssetInput
}

// AssetInput describes one stored rendition of a media item.
type AssetInput struct {
	ID          uuid.UUID
	Rendition   asset.Rendition
	Key         string
	ContentType string
	Size        int64
	Width       int
	Height      int
}

// Draw is a dealt hand with its cards in dealt order. Faces names the media dealt for each card.
type Draw struct {
	*ent.Draw
	Cards []*ent.Card
	// Discard holds the cards drawn earlier in the pass, oldest first.
	Discard []*ent.Card
	// Remaining is how many cards are left to draw in the pass.
	Remaining int
}

// Store persists Prayer Deck data.
type Store interface {
	// ResolveIdentity returns the account for a verified sign-in, linking a new
	// provider to an existing account with the same verified email, or creating
	// an account with a starter deck.
	ResolveIdentity(ctx context.Context, id Identity) (*ent.User, error)
	// User returns an account by ID.
	User(ctx context.Context, userID uuid.UUID) (*ent.User, error)

	// Decks lists the user's decks, oldest first.
	Decks(ctx context.Context, userID uuid.UUID) ([]*Deck, error)
	// CreateDeck adds an empty deck.
	CreateDeck(ctx context.Context, userID uuid.UUID, name string) (*Deck, error)
	// RenameDeck changes a deck's name.
	RenameDeck(ctx context.Context, userID, deckID uuid.UUID, name string) (*Deck, error)
	// DeleteDeck removes a deck and its cards, returning the media keys to remove from storage.
	DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) ([]string, error)
	// Shuffle returns every card in the deck to the draw pool.
	Shuffle(ctx context.Context, userID, deckID uuid.UUID) (*Deck, error)

	// Cards lists a deck's cards with their media newest first, ordered by name.
	Cards(ctx context.Context, userID, deckID uuid.UUID) ([]*ent.Card, error)
	// Card returns a single card with its media newest first.
	Card(ctx context.Context, userID, cardID uuid.UUID) (*ent.Card, error)
	// CreateCard adds a card to a deck.
	CreateCard(ctx context.Context, userID, deckID uuid.UUID, in CardInput) (*ent.Card, error)
	// UpdateCard changes the non-nil fields of a card.
	UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in CardInput) (*ent.Card, error)
	// DeleteCard removes a card, returning the media keys to remove from storage.
	DeleteCard(ctx context.Context, userID, cardID uuid.UUID) ([]string, error)
	// AddMedia adds a media item to a card, making it the card's primary face.
	AddMedia(ctx context.Context, userID, cardID uuid.UUID, in MediaInput) (*ent.Card, error)
	// DeleteMedia removes a card's media item, returning the keys to remove from storage.
	DeleteMedia(ctx context.Context, userID, cardID, mediaID uuid.UUID) ([]string, error)

	// Draw deals up to count cards from the current pass through the deck, choosing
	// one of each card's media at random, and replaces the deck's current draw.
	// With reshuffle set, a fresh pass starts first.
	Draw(ctx context.Context, userID, deckID uuid.UUID, count int, reshuffle bool) (*Draw, error)
	// CurrentDraw returns the deck's most recent draw.
	CurrentDraw(ctx context.Context, userID, deckID uuid.UUID) (*Draw, error)
}

type entStore struct {
	client *ent.Client
	// shuffle randomizes draw order and pick chooses a dealt card's face; tests
	// replace both to make draws deterministic.
	shuffle func(n int, swap func(i, j int))
	pick    func(n int) int
	now     func() time.Time
}

// New returns an ent-backed Store.
func New(client *ent.Client) Store {
	return &entStore{client: client, shuffle: rand.Shuffle, pick: rand.IntN, now: time.Now}
}

func (s *entStore) ResolveIdentity(ctx context.Context, in Identity) (*ent.User, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	linked, err := tx.Identity.Query().
		Where(identity.ProviderEQ(in.Provider), identity.SubjectEQ(in.Subject)).
		QueryUser().
		Only(ctx)
	switch {
	case err == nil:
		return linked, commit(tx)
	case !ent.IsNotFound(err):
		return nil, rollback(tx, fmt.Errorf("failed to query identity: %w", err))
	}

	// Only a provider-verified email may join an existing account; otherwise
	// anyone could claim a collection by registering someone else's address.
	var u *ent.User
	if in.EmailVerified {
		u, err = tx.User.Query().Where(user.EmailEQ(in.Email)).Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, rollback(tx, fmt.Errorf("failed to query user by email: %w", err))
		}
	}
	if u == nil {
		u, err = tx.User.Create().
			SetEmail(in.Email).
			SetName(in.Name).
			SetPictureURL(in.PictureURL).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return nil, rollback(tx, fmt.Errorf("%w: email belongs to another account", ErrConflict))
			}
			return nil, rollback(tx, fmt.Errorf("failed to create user: %w", err))
		}
		if _, err := tx.Deck.Create().SetName(DefaultDeckName).SetOwner(u).SetShuffledAt(s.timestamp()).Save(ctx); err != nil {
			return nil, rollback(tx, fmt.Errorf("failed to create starter deck: %w", err))
		}
	}

	if _, err := tx.Identity.Create().
		SetProvider(in.Provider).
		SetSubject(in.Subject).
		SetEmail(in.Email).
		SetUser(u).
		Save(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to link identity: %w", err))
	}

	return u, commit(tx)
}

func (s *entStore) User(ctx context.Context, userID uuid.UUID) (*ent.User, error) {
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, mapError("failed to get user", err)
	}
	return u, nil
}

func (s *entStore) Decks(ctx context.Context, userID uuid.UUID) ([]*Deck, error) {
	decks, err := s.client.Deck.Query().
		Where(deck.HasOwnerWith(user.ID(userID))).
		WithCards(func(q *ent.CardQuery) { q.Select(card.FieldLastDrawnAt) }).
		Order(ent.Asc(deck.FieldCreatedAt), ent.Asc(deck.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list decks: %w", err)
	}

	result := make([]*Deck, len(decks))
	for i, d := range decks {
		result[i] = summarize(d, d.Edges.Cards)
	}
	return result, nil
}

func (s *entStore) CreateDeck(ctx context.Context, userID uuid.UUID, name string) (*Deck, error) {
	d, err := s.client.Deck.Create().
		SetName(name).
		SetOwnerID(userID).
		SetShuffledAt(s.timestamp()).
		Save(ctx)
	if err != nil {
		return nil, mapError("failed to create deck", err)
	}
	return &Deck{Deck: d}, nil
}

func (s *entStore) RenameDeck(ctx context.Context, userID, deckID uuid.UUID, name string) (*Deck, error) {
	n, err := s.client.Deck.Update().
		Where(deck.ID(deckID), deck.HasOwnerWith(user.ID(userID))).
		SetName(name).
		Save(ctx)
	if err != nil {
		return nil, mapError("failed to rename deck", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.deck(ctx, s.client, userID, deckID)
}

func (s *entStore) DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) ([]string, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	owned, err := tx.Deck.Query().Where(deck.HasOwnerWith(user.ID(userID))).IDs(ctx)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to list decks: %w", err))
	}
	if !slices.Contains(owned, deckID) {
		return nil, rollback(tx, ErrNotFound)
	}
	if len(owned) == 1 {
		return nil, rollback(tx, fmt.Errorf("%w: an account keeps at least one deck", ErrConflict))
	}

	inDeck := asset.HasMediaWith(media.HasCardWith(card.HasDeckWith(deck.ID(deckID))))
	keys, err := tx.Asset.Query().Where(inDeck).Select(asset.FieldKey).Strings(ctx)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to list deck media: %w", err))
	}

	// Children are removed explicitly because sqlite only honors FK cascades
	// when foreign keys are enabled on the connection.
	if _, err := tx.Asset.Delete().Where(inDeck).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete deck assets: %w", err))
	}
	if _, err := tx.Media.Delete().Where(media.HasCardWith(card.HasDeckWith(deck.ID(deckID)))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete deck media: %w", err))
	}
	if _, err := tx.Card.Delete().Where(card.HasDeckWith(deck.ID(deckID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete deck cards: %w", err))
	}
	if _, err := tx.Draw.Delete().Where(draw.HasDeckWith(deck.ID(deckID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete deck draws: %w", err))
	}
	if err := tx.Deck.DeleteOneID(deckID).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete deck: %w", err))
	}

	return keys, commit(tx)
}

func (s *entStore) Shuffle(ctx context.Context, userID, deckID uuid.UUID) (*Deck, error) {
	n, err := s.client.Deck.Update().
		Where(deck.ID(deckID), deck.HasOwnerWith(user.ID(userID))).
		SetShuffledAt(s.timestamp()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to shuffle deck: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.deck(ctx, s.client, userID, deckID)
}

func (s *entStore) Cards(ctx context.Context, userID, deckID uuid.UUID) ([]*ent.Card, error) {
	exists, err := s.client.Deck.Query().Where(deck.ID(deckID), deck.HasOwnerWith(user.ID(userID))).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check deck: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	cards, err := s.client.Card.Query().
		Where(card.HasDeckWith(deck.ID(deckID))).
		WithMedia(withAssets).
		Order(ent.Asc(card.FieldName), ent.Asc(card.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list cards: %w", err)
	}
	return cards, nil
}

func (s *entStore) Card(ctx context.Context, userID, cardID uuid.UUID) (*ent.Card, error) {
	return s.card(ctx, s.client, userID, cardID)
}

func (s *entStore) CreateCard(ctx context.Context, userID, deckID uuid.UUID, in CardInput) (*ent.Card, error) {
	exists, err := s.client.Deck.Query().Where(deck.ID(deckID), deck.HasOwnerWith(user.ID(userID))).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check deck: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	create := s.client.Card.Create().SetDeckID(deckID)
	if in.Name != nil {
		create.SetName(*in.Name)
	}
	if in.Notes != nil {
		create.SetNotes(*in.Notes)
	}
	c, err := create.Save(ctx)
	if err != nil {
		return nil, mapError("failed to create card", err)
	}
	return s.card(ctx, s.client, userID, c.ID)
}

func (s *entStore) UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in CardInput) (*ent.Card, error) {
	update := s.client.Card.Update().Where(card.ID(cardID), ownedCard(userID))
	if in.Name != nil {
		update.SetName(*in.Name)
	}
	if in.Notes != nil {
		update.SetNotes(*in.Notes)
	}
	n, err := update.Save(ctx)
	if err != nil {
		return nil, mapError("failed to update card", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.card(ctx, s.client, userID, cardID)
}

func (s *entStore) DeleteCard(ctx context.Context, userID, cardID uuid.UUID) ([]string, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	c, err := s.card(ctx, tx.Client(), userID, cardID)
	if err != nil {
		return nil, rollback(tx, err)
	}
	keys := mediaKeys(c.Edges.Media)

	if _, err := tx.Asset.Delete().Where(asset.HasMediaWith(media.HasCardWith(card.ID(cardID)))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete card assets: %w", err))
	}
	if _, err := tx.Media.Delete().Where(media.HasCardWith(card.ID(cardID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete card media: %w", err))
	}
	if err := tx.Card.DeleteOneID(cardID).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete card: %w", err))
	}

	return keys, commit(tx)
}

func (s *entStore) AddMedia(ctx context.Context, userID, cardID uuid.UUID, in MediaInput) (*ent.Card, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	if _, err := s.card(ctx, tx.Client(), userID, cardID); err != nil {
		return nil, rollback(tx, err)
	}

	now := s.timestamp()
	if err := tx.Media.Create().
		SetID(in.ID).
		SetCardID(cardID).
		SetKind(in.Kind).
		SetCreatedAt(now).
		Exec(ctx); err != nil {
		return nil, rollback(tx, mapError("failed to save media", err))
	}
	creates := make([]*ent.AssetCreate, len(in.Assets))
	for i, a := range in.Assets {
		creates[i] = tx.Asset.Create().
			SetID(a.ID).
			SetMediaID(in.ID).
			SetRendition(a.Rendition).
			SetKey(a.Key).
			SetContentType(a.ContentType).
			SetSize(a.Size).
			SetWidth(a.Width).
			SetHeight(a.Height).
			SetCreatedAt(now)
	}
	if _, err := tx.Asset.CreateBulk(creates...).Save(ctx); err != nil {
		return nil, rollback(tx, mapError("failed to save media assets", err))
	}
	// Touch the card so clients see the media change in updated_at.
	if err := tx.Card.UpdateOneID(cardID).SetUpdatedAt(now).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to touch card: %w", err))
	}
	if err := commit(tx); err != nil {
		return nil, err
	}

	return s.card(ctx, s.client, userID, cardID)
}

func (s *entStore) DeleteMedia(ctx context.Context, userID, cardID, mediaID uuid.UUID) ([]string, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	m, err := tx.Media.Query().
		Where(media.ID(mediaID), media.HasCardWith(card.ID(cardID), ownedCard(userID))).
		WithAssets().
		Only(ctx)
	if err != nil {
		return nil, rollback(tx, mapError("failed to get media", err))
	}
	keys := mediaKeys([]*ent.Media{m})

	if _, err := tx.Asset.Delete().Where(asset.HasMediaWith(media.ID(mediaID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete media assets: %w", err))
	}
	if err := tx.Media.DeleteOneID(mediaID).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete media: %w", err))
	}
	if err := tx.Card.UpdateOneID(cardID).SetUpdatedAt(s.timestamp()).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to touch card: %w", err))
	}

	return keys, commit(tx)
}

func (s *entStore) Draw(ctx context.Context, userID, deckID uuid.UUID, count int, reshuffle bool) (*Draw, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	d, err := tx.Deck.Query().Where(deck.ID(deckID), deck.HasOwnerWith(user.ID(userID))).Only(ctx)
	if err != nil {
		return nil, rollback(tx, mapError("failed to get deck", err))
	}
	cards, err := tx.Card.Query().
		Where(card.HasDeckWith(deck.ID(deckID))).
		WithMedia(func(q *ent.MediaQuery) { q.Order(newestFirst...) }).
		Order(ent.Asc(card.FieldCreatedAt), ent.Asc(card.FieldID)).
		All(ctx)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to list cards: %w", err))
	}
	if len(cards) == 0 {
		return nil, rollback(tx, ErrEmptyDeck)
	}

	now := s.timestamp()
	shuffledAt := d.ShuffledAt
	if reshuffle {
		shuffledAt = now
		if err := tx.Deck.UpdateOneID(deckID).SetShuffledAt(now).Exec(ctx); err != nil {
			return nil, rollback(tx, fmt.Errorf("failed to shuffle deck: %w", err))
		}
	}

	var pool []*ent.Card
	for _, c := range cards {
		if inPass(c, shuffledAt) {
			pool = append(pool, c)
		}
	}
	if len(pool) == 0 {
		return nil, rollback(tx, ErrPassFinished)
	}

	s.shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	hand := slices.Clone(pool[:min(count, len(pool))])
	remaining := len(pool) - len(hand)

	ids := make([]uuid.UUID, len(hand))
	faces := make(map[uuid.UUID]uuid.UUID, len(hand))
	for i, c := range hand {
		ids[i] = c.ID
		if len(c.Edges.Media) > 0 {
			faces[c.ID] = c.Edges.Media[s.pick(len(c.Edges.Media))].ID
		}
	}
	if err := tx.Card.Update().Where(card.IDIn(ids...)).SetLastDrawnAt(now).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to mark cards drawn: %w", err))
	}
	if _, err := tx.Draw.Delete().Where(draw.HasDeckWith(deck.ID(deckID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to clear previous draw: %w", err))
	}
	dr, err := tx.Draw.Create().SetDeckID(deckID).SetCardIds(ids).SetFaces(faces).SetCreatedAt(now).Save(ctx)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to save draw: %w", err))
	}
	if err := commit(tx); err != nil {
		return nil, err
	}

	dealt, err := s.cardsInOrder(ctx, ids)
	if err != nil {
		return nil, err
	}
	discard, err := s.discard(ctx, deckID, shuffledAt, ids)
	if err != nil {
		return nil, err
	}
	return &Draw{Draw: dr, Cards: dealt, Discard: discard, Remaining: remaining}, nil
}

func (s *entStore) CurrentDraw(ctx context.Context, userID, deckID uuid.UUID) (*Draw, error) {
	d, err := s.deck(ctx, s.client, userID, deckID)
	if err != nil {
		return nil, err
	}

	dr, err := s.client.Draw.Query().
		Where(draw.HasDeckWith(deck.ID(deckID))).
		Order(ent.Desc(draw.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		return nil, mapError("failed to get current draw", err)
	}

	dealt, err := s.cardsInOrder(ctx, dr.CardIds)
	if err != nil {
		return nil, err
	}
	discard, err := s.discard(ctx, deckID, d.ShuffledAt, dr.CardIds)
	if err != nil {
		return nil, err
	}
	return &Draw{Draw: dr, Cards: dealt, Discard: discard, Remaining: d.Remaining}, nil
}

// discard loads the cards drawn in the pass that began at shuffledAt, oldest
// first, leaving out the hand on the table.
func (s *entStore) discard(ctx context.Context, deckID uuid.UUID, shuffledAt time.Time, hand []uuid.UUID) ([]*ent.Card, error) {
	cards, err := s.client.Card.Query().
		Where(card.HasDeckWith(deck.ID(deckID)), card.LastDrawnAtGTE(shuffledAt), card.IDNotIn(hand...)).
		WithMedia(withAssets).
		Order(ent.Asc(card.FieldLastDrawnAt), ent.Asc(card.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load discarded cards: %w", err)
	}
	return cards, nil
}

// cardsInOrder loads cards with media in the order of ids, skipping any since deleted.
func (s *entStore) cardsInOrder(ctx context.Context, ids []uuid.UUID) ([]*ent.Card, error) {
	cards, err := s.client.Card.Query().Where(card.IDIn(ids...)).WithMedia(withAssets).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load drawn cards: %w", err)
	}
	byID := make(map[uuid.UUID]*ent.Card, len(cards))
	for _, c := range cards {
		byID[c.ID] = c
	}
	ordered := make([]*ent.Card, 0, len(ids))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			ordered = append(ordered, c)
		}
	}
	return ordered, nil
}

func (s *entStore) deck(ctx context.Context, client *ent.Client, userID, deckID uuid.UUID) (*Deck, error) {
	d, err := client.Deck.Query().
		Where(deck.ID(deckID), deck.HasOwnerWith(user.ID(userID))).
		WithCards(func(q *ent.CardQuery) { q.Select(card.FieldLastDrawnAt) }).
		Only(ctx)
	if err != nil {
		return nil, mapError("failed to get deck", err)
	}
	return summarize(d, d.Edges.Cards), nil
}

func (s *entStore) card(ctx context.Context, client *ent.Client, userID, cardID uuid.UUID) (*ent.Card, error) {
	c, err := client.Card.Query().
		Where(card.ID(cardID), ownedCard(userID)).
		WithMedia(withAssets).
		Only(ctx)
	if err != nil {
		return nil, mapError("failed to get card", err)
	}
	return c, nil
}

// timestamp is truncated to the microsecond Postgres stores, so a time written
// and read back compares equal to the one still held in memory.
func (s *entStore) timestamp() time.Time {
	return s.now().UTC().Truncate(time.Microsecond)
}

// newestFirst orders a card's media so its primary face comes first.
var newestFirst = []media.OrderOption{ent.Desc(media.FieldCreatedAt), ent.Desc(media.FieldID)}

// withAssets loads a card's media newest first, each with its renditions.
func withAssets(q *ent.MediaQuery) {
	q.WithAssets().Order(newestFirst...)
}

func ownedCard(userID uuid.UUID) predicate.Card {
	return card.HasDeckWith(deck.HasOwnerWith(user.ID(userID)))
}

// inPass reports whether a card is still waiting to be drawn in the pass that began at shuffledAt.
func inPass(c *ent.Card, shuffledAt time.Time) bool {
	return c.LastDrawnAt == nil || c.LastDrawnAt.Before(shuffledAt)
}

func summarize(d *ent.Deck, cards []*ent.Card) *Deck {
	remaining := 0
	for _, c := range cards {
		if inPass(c, d.ShuffledAt) {
			remaining++
		}
	}
	d.Edges.Cards = nil
	return &Deck{Deck: d, CardCount: len(cards), Remaining: remaining}
}

func mediaKeys(items []*ent.Media) []string {
	keys := []string{}
	for _, m := range items {
		for _, a := range m.Edges.Assets {
			keys = append(keys, a.Key)
		}
	}
	return keys
}

func mapError(msg string, err error) error {
	switch {
	case ent.IsNotFound(err):
		return ErrNotFound
	case ent.IsValidationError(err):
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	case ent.IsConstraintError(err):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	default:
		return fmt.Errorf("%s: %w", msg, err)
	}
}

func commit(tx *ent.Tx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func rollback(tx *ent.Tx, err error) error {
	if rerr := tx.Rollback(); rerr != nil {
		return errors.Join(err, fmt.Errorf("failed to rollback transaction: %w", rerr))
	}
	return err
}

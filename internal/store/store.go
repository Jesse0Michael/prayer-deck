// Package store is the persistence seam for Prayer Deck: accounts, decks,
// cards, artwork records, and draws, backed by ent. Every read and write is
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
	// Remaining is how many cards are left before the deck reshuffles.
	Remaining int
}

// CardInput carries card fields to create or change. Nil fields are left unchanged on update.
type CardInput struct {
	Name    *string
	Kind    *card.Kind
	Details *string
}

// AssetInput describes one stored artwork rendition.
type AssetInput struct {
	ID          uuid.UUID
	Rendition   asset.Rendition
	Key         string
	ContentType string
	Size        int64
	Width       int
	Height      int
}

// Draw is a dealt hand with its cards in dealt order.
type Draw struct {
	*ent.Draw
	Cards []*ent.Card
	// Remaining is how many cards are left before the deck reshuffles.
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
	// DeleteDeck removes a deck and its cards, returning the artwork keys to remove from storage.
	DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) ([]string, error)
	// Shuffle returns every card in the deck to the draw pool.
	Shuffle(ctx context.Context, userID, deckID uuid.UUID) (*Deck, error)

	// Cards lists a deck's cards with artwork, ordered by name.
	Cards(ctx context.Context, userID, deckID uuid.UUID) ([]*ent.Card, error)
	// Card returns a single card with artwork.
	Card(ctx context.Context, userID, cardID uuid.UUID) (*ent.Card, error)
	// CreateCard adds a card to a deck.
	CreateCard(ctx context.Context, userID, deckID uuid.UUID, in CardInput) (*ent.Card, error)
	// UpdateCard changes the non-nil fields of a card.
	UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in CardInput) (*ent.Card, error)
	// DeleteCard removes a card, returning the artwork keys to remove from storage.
	DeleteCard(ctx context.Context, userID, cardID uuid.UUID) ([]string, error)
	// ReplaceArtwork swaps a card's artwork for the given renditions, returning
	// the card and the replaced artwork keys to remove from storage.
	ReplaceArtwork(ctx context.Context, userID, cardID uuid.UUID, assets []AssetInput) (*ent.Card, []string, error)

	// Draw deals up to count cards from the current pass through the deck,
	// reshuffling when the pass runs out, and replaces the deck's current draw.
	Draw(ctx context.Context, userID, deckID uuid.UUID, count int) (*Draw, error)
	// CurrentDraw returns the deck's most recent draw.
	CurrentDraw(ctx context.Context, userID, deckID uuid.UUID) (*Draw, error)
}

type entStore struct {
	client *ent.Client
	// shuffle randomizes draw order; tests replace it to make draws deterministic.
	shuffle func(n int, swap func(i, j int))
	now     func() time.Time
}

// New returns an ent-backed Store.
func New(client *ent.Client) Store {
	return &entStore{client: client, shuffle: rand.Shuffle, now: time.Now}
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

	keys, err := tx.Asset.Query().
		Where(asset.HasCardWith(card.HasDeckWith(deck.ID(deckID)))).
		Select(asset.FieldKey).
		Strings(ctx)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to list deck artwork: %w", err))
	}

	// Children are removed explicitly because sqlite only honors FK cascades
	// when foreign keys are enabled on the connection.
	if _, err := tx.Asset.Delete().Where(asset.HasCardWith(card.HasDeckWith(deck.ID(deckID)))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete deck artwork: %w", err))
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
		WithAssets().
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
	if in.Kind != nil {
		create.SetKind(*in.Kind)
	}
	if in.Details != nil {
		create.SetDetails(*in.Details)
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
	if in.Kind != nil {
		update.SetKind(*in.Kind)
	}
	if in.Details != nil {
		update.SetDetails(*in.Details)
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
	keys := assetKeys(c.Edges.Assets)

	if _, err := tx.Asset.Delete().Where(asset.HasCardWith(card.ID(cardID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete card artwork: %w", err))
	}
	if err := tx.Card.DeleteOneID(cardID).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to delete card: %w", err))
	}

	return keys, commit(tx)
}

func (s *entStore) ReplaceArtwork(ctx context.Context, userID, cardID uuid.UUID, assets []AssetInput) (*ent.Card, []string, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open transaction: %w", err)
	}

	c, err := s.card(ctx, tx.Client(), userID, cardID)
	if err != nil {
		return nil, nil, rollback(tx, err)
	}
	replaced := assetKeys(c.Edges.Assets)

	if _, err := tx.Asset.Delete().Where(asset.HasCardWith(card.ID(cardID))).Exec(ctx); err != nil {
		return nil, nil, rollback(tx, fmt.Errorf("failed to delete previous artwork: %w", err))
	}
	creates := make([]*ent.AssetCreate, len(assets))
	for i, a := range assets {
		creates[i] = tx.Asset.Create().
			SetID(a.ID).
			SetCardID(cardID).
			SetRendition(a.Rendition).
			SetKey(a.Key).
			SetContentType(a.ContentType).
			SetSize(a.Size).
			SetWidth(a.Width).
			SetHeight(a.Height)
	}
	if _, err := tx.Asset.CreateBulk(creates...).Save(ctx); err != nil {
		return nil, nil, rollback(tx, fmt.Errorf("failed to save artwork: %w", err))
	}
	// Touch the card so clients see the artwork change in updated_at.
	if err := tx.Card.UpdateOneID(cardID).SetUpdatedAt(s.timestamp()).Exec(ctx); err != nil {
		return nil, nil, rollback(tx, fmt.Errorf("failed to touch card: %w", err))
	}
	if err := commit(tx); err != nil {
		return nil, nil, err
	}

	c, err = s.card(ctx, s.client, userID, cardID)
	if err != nil {
		return nil, nil, err
	}
	return c, replaced, nil
}

func (s *entStore) Draw(ctx context.Context, userID, deckID uuid.UUID, count int) (*Draw, error) {
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
		Order(ent.Asc(card.FieldCreatedAt), ent.Asc(card.FieldID)).
		All(ctx)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to list cards: %w", err))
	}
	if len(cards) == 0 {
		return nil, rollback(tx, ErrEmptyDeck)
	}

	var pool, drawn []*ent.Card
	for _, c := range cards {
		if inPass(c, d.ShuffledAt) {
			pool = append(pool, c)
		} else {
			drawn = append(drawn, c)
		}
	}

	n := min(count, len(cards))
	now := s.timestamp()
	s.shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	hand := slices.Clone(pool[:min(n, len(pool))])
	remaining := len(pool) - len(hand)

	// When the pass runs out mid-hand, the deck reshuffles and the hand is
	// filled from cards drawn earlier in the finished pass.
	if len(hand) < n {
		s.shuffle(len(drawn), func(i, j int) { drawn[i], drawn[j] = drawn[j], drawn[i] })
		hand = append(hand, drawn[:n-len(hand)]...)
		remaining = len(cards) - len(hand)
		if err := tx.Deck.UpdateOneID(deckID).SetShuffledAt(now).Exec(ctx); err != nil {
			return nil, rollback(tx, fmt.Errorf("failed to reshuffle deck: %w", err))
		}
	}

	ids := make([]uuid.UUID, len(hand))
	for i, c := range hand {
		ids[i] = c.ID
	}
	if err := tx.Card.Update().Where(card.IDIn(ids...)).SetLastDrawnAt(now).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to mark cards drawn: %w", err))
	}
	if _, err := tx.Draw.Delete().Where(draw.HasDeckWith(deck.ID(deckID))).Exec(ctx); err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to clear previous draw: %w", err))
	}
	dr, err := tx.Draw.Create().SetDeckID(deckID).SetCardIds(ids).SetCreatedAt(now).Save(ctx)
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
	return &Draw{Draw: dr, Cards: dealt, Remaining: remaining}, nil
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
	return &Draw{Draw: dr, Cards: dealt, Remaining: d.Remaining}, nil
}

// cardsInOrder loads cards with artwork in the order of ids, skipping any since deleted.
func (s *entStore) cardsInOrder(ctx context.Context, ids []uuid.UUID) ([]*ent.Card, error) {
	cards, err := s.client.Card.Query().Where(card.IDIn(ids...)).WithAssets().All(ctx)
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
		WithAssets().
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

func assetKeys(assets []*ent.Asset) []string {
	keys := make([]string, len(assets))
	for i, a := range assets {
		keys[i] = a.Key
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

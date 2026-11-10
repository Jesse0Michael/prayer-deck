package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Draw is the hand most recently dealt from a deck, kept so a prayer time can
// be resumed after closing the app. Only the latest draw per deck is retained.
type Draw struct {
	ent.Schema
}

func (Draw) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		// card_ids is the dealt order. Cards deleted after the draw are skipped
		// when the draw is read.
		field.JSON("card_ids", []uuid.UUID{}).
			Immutable(),
		// faces maps each dealt card to the media chosen for it, so a resumed
		// draw shows the same pictures. Cards without media are absent.
		field.JSON("faces", map[uuid.UUID]uuid.UUID{}).
			Optional().
			Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Draw) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("deck", Deck.Type).
			Ref("draws").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

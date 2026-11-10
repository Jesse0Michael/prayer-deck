package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Card represents the people held in prayer: a person, a couple, or a family.
type Card struct {
	ent.Schema
}

func (Card) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("name").
			NotEmpty(),
		// notes holds the small personal touches that bring someone to mind:
		// a hobby, a pet, a favorite place.
		field.Text("notes").
			Optional(),
		field.Time("last_drawn_at").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Card) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("deck", Deck.Type).
			Ref("cards").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("media", Media.Type),
	}
}

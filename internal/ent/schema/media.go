package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Media is one picture of the people on a card. A card holds many; the newest
// is its primary face, and a draw deals any one of them.
type Media struct {
	ent.Schema
}

func (Media) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		// kind leaves room for video loops beside still images.
		field.Enum("kind").
			Values("image").
			Immutable(),
		// created_at orders a card's media, newest first.
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Media) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("card", Card.Type).
			Ref("media").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("assets", Asset.Type),
	}
}

func (Media) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at").Edges("card"),
	}
}

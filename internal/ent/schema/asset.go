package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Asset is one stored media file of a card's artwork. A single upload produces
// several renditions, each sized for where it is shown.
type Asset struct {
	ent.Schema
}

func (Asset) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		// rendition is the role this file plays: the untouched upload, the
		// card-face image, or the small thumbnail for lists and preloading.
		field.Enum("rendition").
			Values("original", "display", "thumbnail").
			Immutable(),
		// key is the object key in the media bucket.
		field.String("key").
			Unique().
			Immutable(),
		field.String("content_type").
			Immutable(),
		field.Int64("size").
			Immutable(),
		field.Int("width").
			Immutable(),
		field.Int("height").
			Immutable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Asset) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("card", Card.Type).
			Ref("assets").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

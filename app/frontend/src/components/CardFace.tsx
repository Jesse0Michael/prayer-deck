import type { Card } from "../api/types";

/** A card's front: its artwork, or a handwritten name on blank card stock while artwork is missing. */
export function CardFace({ card, thumbnail = false }: { card: Card; thumbnail?: boolean }) {
  if (card.artwork) {
    return (
      <div className="card-face">
        <img
          src={thumbnail ? card.artwork.thumbnail_url : card.artwork.display_url}
          alt={card.name}
          draggable={false}
          decoding="async"
        />
      </div>
    );
  }
  return (
    <div className="card-face card-face-blank">
      <span className="hand-name">{card.name}</span>
    </div>
  );
}

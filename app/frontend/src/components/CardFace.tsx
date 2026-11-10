import type { DealtCard, Media } from "../api/types";
import { faceOf } from "../lib/media";

interface CardFaceProps {
  card: DealtCard;
  /** The picture to show; defaults to the card's dealt face or primary media. */
  media?: Media | null;
  thumbnail?: boolean;
}

/** A card's front: its picture, or a handwritten name on blank card stock while it has none. */
export function CardFace({ card, media = faceOf(card), thumbnail = false }: CardFaceProps) {
  if (media) {
    return (
      <div className="card-face">
        <img src={thumbnail ? media.thumbnail_url : media.display_url} alt={card.name} draggable={false} decoding="async" />
      </div>
    );
  }
  return (
    <div className="card-face card-face-blank">
      <span className="hand-name">{card.name}</span>
    </div>
  );
}

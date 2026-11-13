import cardBackUrl from "../assets/card-back.webp";

/** The shared back of every card: warm card stock with a hand-drawn cross framed by vines. */
export function CardBack() {
  return (
    <div className="card-back">
      <img src={cardBackUrl} alt="" draggable={false} decoding="async" />
    </div>
  );
}

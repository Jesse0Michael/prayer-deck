import { AnimatePresence, motion, type PanInfo } from "motion/react";
import { useEffect, useState } from "react";
import type { Card } from "../api/types";
import { PlayingCard } from "../components/PlayingCard";

interface FocusCardProps {
  cards: Card[];
  index: number;
  onIndexChange: (index: number) => void;
  onClose: () => void;
}

const kindPhrase: Record<Card["kind"], string> = {
  person: "",
  couple: "together",
  family: "and their family",
};

// Inline because the CSS minifier keeps only the -webkit- form of backdrop-filter.
const blur = { backdropFilter: "blur(10px)", WebkitBackdropFilter: "blur(10px)" };

function useFocusCardWidth(): number {
  const measure = () => Math.min(window.innerWidth * 0.78, (window.innerHeight * 0.62) / 1.5, 420);
  const [width, setWidth] = useState(measure);
  useEffect(() => {
    const onResize = () => setWidth(measure());
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);
  return width;
}

/** A drawn card held up close, with room for the details that bring the person to mind. */
export function FocusCard({ cards, index, onIndexChange, onClose }: FocusCardProps) {
  const card = cards[index];
  const width = useFocusCardWidth();
  const [direction, setDirection] = useState(0);

  const go = (step: number) => {
    const next = index + step;
    if (next < 0 || next >= cards.length) return;
    setDirection(step);
    onIndexChange(next);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowRight") go(1);
      if (e.key === "ArrowLeft") go(-1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const onDragEnd = (_: unknown, info: PanInfo) => {
    if (info.offset.x < -60 || info.velocity.x < -400) go(1);
    else if (info.offset.x > 60 || info.velocity.x > 400) go(-1);
  };

  return (
    <motion.div
      className="focus-backdrop"
      style={blur}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.3 }}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={card.name}
    >
      <div className="focus-layout" onClick={(e) => e.stopPropagation()}>
        <AnimatePresence mode="popLayout" initial={false} custom={direction}>
          <motion.div
            key={card.id}
            className="focus-card"
            custom={direction}
            variants={{
              enter: (d: number) => ({ x: d * 80, opacity: 0, rotate: d * 4 }),
              center: { x: 0, opacity: 1, rotate: 0 },
              exit: (d: number) => ({ x: d * -80, opacity: 0, rotate: d * -4 }),
            }}
            initial="enter"
            animate="center"
            exit="exit"
            transition={{ type: "spring", stiffness: 160, damping: 22 }}
            drag={cards.length > 1 ? "x" : false}
            dragConstraints={{ left: 0, right: 0 }}
            dragElastic={0.35}
            onDragEnd={onDragEnd}
          >
            <motion.div initial={{ scale: 0.86, y: 30 }} animate={{ scale: 1, y: 0 }} transition={{ type: "spring", stiffness: 140, damping: 18 }}>
              <PlayingCard card={card} faceUp width={width} />
            </motion.div>
          </motion.div>
        </AnimatePresence>

        <motion.div
          key={`${card.id}-notes`}
          className="focus-notes"
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15, duration: 0.35 }}
        >
          <h2 className="hand-name">{card.name}</h2>
          {card.details && <p className="focus-details">{card.details}</p>}
          <p className="focus-prompt">
            Take a moment to hold {[card.name, kindPhrase[card.kind]].filter(Boolean).join(" ")} in prayer.
          </p>
          <div className="focus-nav">
            <button type="button" className="btn btn-quiet" onClick={() => go(-1)} disabled={index === 0} aria-label="Previous card">
              ←
            </button>
            <span className="focus-count">
              {index + 1} of {cards.length}
            </span>
            <button
              type="button"
              className="btn btn-quiet"
              onClick={() => go(1)}
              disabled={index === cards.length - 1}
              aria-label="Next card"
            >
              →
            </button>
          </div>
          <button type="button" className="btn" onClick={onClose}>
            Back to the table
          </button>
        </motion.div>
      </div>
    </motion.div>
  );
}

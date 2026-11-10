import { motion } from "motion/react";
import { useEffect, useState } from "react";
import type { ApiClient } from "../api/client";
import type { DealtCard } from "../api/types";
import { NotesPanel } from "../components/NotesPanel";
import { PlayingCard } from "../components/PlayingCard";

interface FocusCardProps {
  api: ApiClient;
  card: DealtCard;
  onClose: () => void;
}

const NOTES_PAGE_SIZE = 3;

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

/** A drawn card held up close, with room for the notes that bring the person to mind. */
export function FocusCard({ api, card, onClose }: FocusCardProps) {
  const width = useFocusCardWidth();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

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
        <motion.div
          className="focus-card"
          initial={{ scale: 0.86, y: 30, opacity: 0 }}
          animate={{ scale: 1, y: 0, opacity: 1 }}
          transition={{ type: "spring", stiffness: 140, damping: 18 }}
        >
          <PlayingCard card={card} faceUp width={width} />
        </motion.div>

        <motion.div
          className="focus-notes"
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15, duration: 0.35 }}
        >
          <h2 className="hand-name">{card.name}</h2>
          <NotesPanel api={api} cardID={card.id} pageSize={NOTES_PAGE_SIZE} />
          <button type="button" className="btn" onClick={onClose}>
            Back to the table
          </button>
        </motion.div>
      </div>
    </motion.div>
  );
}

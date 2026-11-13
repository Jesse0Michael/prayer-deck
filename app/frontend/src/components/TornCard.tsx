import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { ApiError } from "../api/client";
import tornBottomUrl from "../assets/card-torn-bottom.webp";
import tornTopUrl from "../assets/card-torn-top.webp";
import { preloadImage } from "../lib/media";

// Inline because the CSS minifier keeps only the -webkit- form of backdrop-filter.
const blur = { backdropFilter: "blur(4px)", WebkitBackdropFilter: "blur(4px)" };

// Offsets are percentages of the card. The halves drift together, nearly meet with a
// small press at the seam, and fall apart again, so the card never quite mends.
const MEND_TIMES = [0, 0.38, 0.46, 0.54, 0.62, 1];
const MEND = { duration: 3.2, times: MEND_TIMES, ease: "easeInOut", repeat: Infinity, repeatDelay: 0.6 } as const;
const topApart = { x: "-3%", y: "-9%", rotate: -6 };
const bottomApart = { x: "3%", y: "9%", rotate: 5 };
const topMend = {
  x: ["-3%", "-1%", "-0.6%", "-0.9%", "-0.6%", "-3%"],
  y: ["-9%", "-2.4%", "-1.4%", "-2%", "-1.5%", "-9%"],
  rotate: [-6, -1.8, -1, -1.6, -1.1, -6],
};
const bottomMend = {
  x: ["3%", "0.8%", "0.5%", "0.8%", "0.5%", "3%"],
  y: ["9%", "2.4%", "1.4%", "2%", "1.5%", "9%"],
  rotate: [5, 1.5, 0.8, 1.3, 0.9, 5],
};

interface TornCardProps {
  /** The failure to report; the overlay is hidden while null. */
  error: ApiError | null;
  onDismiss: () => void;
}

/** Reports a failed server request with a torn card whose halves keep trying to rejoin. */
export function TornCard({ error, onDismiss }: TornCardProps) {
  return createPortal(<AnimatePresence>{error && <TornCardDialog error={error} onDismiss={onDismiss} />}</AnimatePresence>, document.body);
}

function TornCardDialog({ error, onDismiss }: { error: ApiError; onDismiss: () => void }) {
  const titleID = useId();
  const messageID = useId();
  const reduceMotion = useReducedMotion();
  const dismissRef = useRef<HTMLButtonElement>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([preloadImage(tornTopUrl), preloadImage(tornBottomUrl)]).then(() => !cancelled && setReady(true));
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    dismissRef.current?.focus();
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onDismiss();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onDismiss]);

  const serverSide = error.status >= 500;

  return (
    <motion.div
      className="torn-backdrop"
      style={blur}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onDismiss}
    >
      <motion.div
        className="torn-dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={messageID}
        onClick={(e) => e.stopPropagation()}
        initial={{ y: 30, opacity: 0, scale: 0.98 }}
        animate={{ y: 0, opacity: 1, scale: 1 }}
        exit={{ y: 20, opacity: 0, scale: 0.98 }}
        transition={{ type: "spring", stiffness: 220, damping: 26 }}
      >
        <div className="torn-stage" aria-hidden="true">
          {ready && (
            <motion.div
              className="torn-card"
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ type: "spring", stiffness: 180, damping: 20 }}
            >
              <motion.img
                src={tornTopUrl}
                alt=""
                draggable={false}
                initial={topApart}
                animate={reduceMotion ? topApart : topMend}
                transition={MEND}
              />
              <motion.img
                src={tornBottomUrl}
                alt=""
                draggable={false}
                initial={bottomApart}
                animate={reduceMotion ? bottomApart : bottomMend}
                transition={MEND}
              />
            </motion.div>
          )}
        </div>
        <h2 id={titleID} className="hand-title">
          Something tore
        </h2>
        <p id={messageID}>
          {serverSide ? "The server stumbled on that. Give it a moment and try again." : "That request didn't go through."}
        </p>
        <p className="torn-detail">
          {error.status} · {error.message}
        </p>
        <button ref={dismissRef} type="button" className="btn btn-primary" onClick={onDismiss}>
          Okay
        </button>
      </motion.div>
    </motion.div>
  );
}

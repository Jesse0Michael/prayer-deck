import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ApiError, type ApiClient } from "../api/client";
import type { Deck, Draw } from "../api/types";
import { CardBack } from "../components/CardBack";
import { PlayingCard } from "../components/PlayingCard";
import { preloadFaces } from "../lib/media";
import { useElementSize, type Size } from "../lib/useElementSize";
import { FocusCard } from "./FocusCard";

type Phase = "loading" | "idle" | "shuffling" | "dealing" | "dealt" | "collecting";

const DRAW_COUNT = 3;
const STACK_DEPTH = 7;
const SHUFFLE_MS = 1500;
const DEAL_STAGGER = 0.22;
const COLLECT_MS = 650;
const REVEALED_KEY = "prayer-deck.revealed";

const dealSpring = { type: "spring", stiffness: 120, damping: 17, mass: 1 } as const;

interface Placement {
  x: number;
  y: number;
  scale: number;
}

/** Positions the deck and hand within the stage so every card moves in one coordinate space. */
function computeLayout({ width, height }: Size, count: number) {
  const pad = 16;
  const gap = width < 560 ? 10 : 28;
  const n = Math.max(count, 1);
  const handW = Math.max(64, Math.min((width - pad * 2 - gap * (n - 1)) / n, (height * 0.6) / 1.5, 270));
  const handH = handW * 1.5;
  const deckW = Math.max(80, Math.min(handW * 1.15, (height * 0.68) / 1.5, 280));
  const smallScale = 0.4;
  const deckSmallH = deckW * 1.5 * smallScale;
  const rowGap = 30;
  const top = Math.max(pad, (height - (deckSmallH + rowGap + handH)) / 2);

  const deckIdle: Placement = { x: width / 2, y: height / 2, scale: 1 };
  const deckAside: Placement = { x: width / 2, y: top + deckSmallH / 2, scale: smallScale };
  const handY = top + deckSmallH + rowGap + handH / 2;
  const slot = (i: number): Placement => ({ x: width / 2 + (i - (n - 1) / 2) * (handW + gap), y: handY, scale: 1 });
  return { handW, handH, deckW, deckIdle, deckAside, slot };
}

function loadRevealed(drawID: string): Set<string> {
  try {
    const saved = JSON.parse(localStorage.getItem(REVEALED_KEY) ?? "null") as { drawID: string; cardIDs: string[] } | null;
    if (saved?.drawID === drawID) return new Set(saved.cardIDs);
  } catch {
    // A corrupt entry just means nothing is revealed.
  }
  return new Set();
}

function saveRevealed(drawID: string, revealed: Set<string>) {
  localStorage.setItem(REVEALED_KEY, JSON.stringify({ drawID, cardIDs: [...revealed] }));
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

interface DrawViewProps {
  api: ApiClient;
  deck: Deck;
  onDeckChanged: () => void;
  onManage: () => void;
}

export function DrawView({ api, deck, onDeckChanged, onManage }: DrawViewProps) {
  const stageRef = useRef<HTMLDivElement>(null);
  const size = useElementSize(stageRef);
  const [phase, setPhase] = useState<Phase>("loading");
  const [draw, setDraw] = useState<Draw | null>(null);
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [focusIndex, setFocusIndex] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPhase("loading");
    setDraw(null);
    setFocusIndex(null);
    setError(null);
    api
      .currentDraw(deck.id)
      .then(async (current) => {
        if (cancelled) return;
        if (current && current.cards.length > 0) {
          await preloadFaces(current.cards, 3000);
          if (cancelled) return;
          setDraw(current);
          setRevealed(loadRevealed(current.id));
          setPhase("dealt");
        } else {
          setPhase("idle");
        }
      })
      .catch((err: Error) => {
        if (cancelled) return;
        setError(err.message);
        setPhase("idle");
      });
    return () => {
      cancelled = true;
    };
  }, [api, deck.id]);

  const count = draw?.cards.length ?? DRAW_COUNT;
  const layout = useMemo(() => computeLayout(size, count), [size, count]);
  const handOut = phase === "dealing" || phase === "dealt" || phase === "collecting";
  const busy = phase === "loading" || phase === "shuffling" || phase === "dealing" || phase === "collecting";
  const deckPlacement = handOut ? layout.deckAside : layout.deckIdle;
  const deckAsHand: Placement = { ...deckPlacement, scale: (layout.deckW * deckPlacement.scale) / layout.handW };

  const startDraw = useCallback(async () => {
    if (busy) return;
    setError(null);
    setFocusIndex(null);
    if (draw) {
      setPhase("collecting");
      await sleep(COLLECT_MS + DEAL_STAGGER * 1000 * (draw.cards.length - 1));
      setDraw(null);
    }
    setPhase("shuffling");
    try {
      const [dealt] = await Promise.all([
        api.draw(deck.id, DRAW_COUNT).then(async (d) => {
          await preloadFaces(d.cards);
          return d;
        }),
        sleep(SHUFFLE_MS),
      ]);
      setDraw(dealt);
      setRevealed(new Set());
      saveRevealed(dealt.id, new Set());
      setPhase("dealing");
      onDeckChanged();
      await sleep(DEAL_STAGGER * 1000 * (dealt.cards.length - 1) + 700);
      setPhase("dealt");
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        setError("empty");
      } else {
        setError(err instanceof Error ? err.message : "Something went wrong");
      }
      setPhase("idle");
    }
  }, [api, busy, deck.id, draw, onDeckChanged]);

  const reveal = useCallback(
    (cardIDs: string[]) => {
      if (!draw) return;
      setRevealed((prev) => {
        const next = new Set(prev);
        cardIDs.forEach((id) => next.add(id));
        saveRevealed(draw.id, next);
        return next;
      });
    },
    [draw],
  );

  const allRevealed = draw ? draw.cards.every((c) => revealed.has(c.id)) : false;

  return (
    <div className="draw-view">
      <div className="stage" ref={stageRef}>
        {size.width > 0 && (
          <>
            <motion.div
              className="deck"
              style={{ width: layout.deckW, height: layout.deckW * 1.5, left: -layout.deckW / 2, top: (-layout.deckW * 1.5) / 2 }}
              initial={false}
              animate={{ ...deckPlacement }}
              transition={{ type: "spring", stiffness: 150, damping: 22 }}
            >
              <button
                type="button"
                className={`deck-button${phase === "idle" ? " deck-ready" : ""}`}
                onClick={startDraw}
                disabled={busy || phase === "dealt"}
                aria-label="Shuffle and draw three cards"
              >
                {Array.from({ length: STACK_DEPTH }, (_, i) => (
                  <ShuffleCard key={i} index={i} width={layout.deckW} shuffling={phase === "shuffling"} />
                ))}
              </button>
            </motion.div>

            {draw &&
                draw.cards.map((card, i) => {
                  const target = phase === "collecting" ? deckAsHand : layout.slot(i);
                  const order = phase === "collecting" ? draw.cards.length - 1 - i : i;
                  return (
                    <motion.div
                      key={`${draw.id}-${card.id}`}
                      className="hand-card"
                      style={{ width: layout.handW, left: -layout.handW / 2, top: -layout.handH / 2, zIndex: 10 + i }}
                      initial={{ ...deckAsHand, rotate: -4 + i * 3 }}
                      animate={{ ...target, rotate: phase === "collecting" ? 0 : (i - (draw.cards.length - 1) / 2) * 1.5 }}
                      transition={{ ...dealSpring, delay: phase === "dealt" ? 0 : order * DEAL_STAGGER }}
                    >
                      <PlayingCard
                        card={card}
                        width={layout.handW}
                        faceUp={revealed.has(card.id) && phase !== "collecting"}
                        tactile={phase === "dealt"}
                        label={revealed.has(card.id) ? `Spend time with ${card.name}` : "Turn card over"}
                        onActivate={
                          phase === "dealt" ? () => (revealed.has(card.id) ? setFocusIndex(i) : reveal([card.id])) : undefined
                        }
                      />
                    </motion.div>
                  );
                })}
          </>
        )}
      </div>

      <div className="draw-controls">
        <AnimatePresence mode="wait">
          <motion.div
            key={error === "empty" ? "empty" : phase === "dealt" ? (allRevealed ? "revealed" : "dealt") : phase}
            className="draw-hint"
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.25 }}
          >
            {error === "empty" ? (
              <>
                <p>This deck has no cards yet.</p>
                <button type="button" className="btn btn-primary" onClick={onManage}>
                  Add cards
                </button>
              </>
            ) : phase === "idle" ? (
              <button type="button" className="btn btn-primary" onClick={startDraw}>
                Shuffle &amp; draw
              </button>
            ) : phase === "shuffling" ? (
              <p>Shuffling…</p>
            ) : phase === "dealt" && allRevealed ? (
              <button type="button" className="btn" onClick={startDraw}>
                Draw again
              </button>
            ) : (
              <p>&nbsp;</p>
            )}
          </motion.div>
        </AnimatePresence>
        {error && error !== "empty" && <p className="error-text">{error}</p>}
      </div>

      {createPortal(
        <AnimatePresence>
          {draw && focusIndex !== null && (
            <FocusCard card={draw.cards[focusIndex]} onClose={() => setFocusIndex(null)} />
          )}
        </AnimatePresence>,
        document.body,
      )}
    </div>
  );
}

/** One card in the deck stack; while shuffling, alternate cards riffle out to either side and back. */
function ShuffleCard({ index, width, shuffling }: { index: number; width: number; shuffling: boolean }) {
  const side = index % 2 === 0 ? -1 : 1;
  const rest = { x: 0, y: -index * 1.4, rotate: ((index * 37) % 7) - 3, zIndex: index };
  const spread = width * 0.62;
  return (
    <motion.div
      className="stack-card"
      initial={false}
      animate={
        shuffling
          ? {
              x: [0, side * spread, 0, side * spread, 0],
              y: [rest.y, rest.y - 10, rest.y, rest.y - 10, rest.y],
              rotate: [rest.rotate, side * -9, rest.rotate, side * -9, rest.rotate],
              zIndex: [index, side > 0 ? index : index + STACK_DEPTH, index, side > 0 ? index + STACK_DEPTH : index, index],
            }
          : rest
      }
      transition={
        shuffling
          ? { duration: SHUFFLE_MS / 1000, times: [0, 0.25, 0.5, 0.75, 1], ease: "easeInOut", delay: index * 0.012 }
          : { type: "spring", stiffness: 200, damping: 20 }
      }
    >
      <CardBack />
    </motion.div>
  );
}

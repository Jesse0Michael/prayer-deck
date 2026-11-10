import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ApiError } from "../api/client";
import type { DeckApi } from "../api/deckApi";
import type { DealtCard, Deck, Draw } from "../api/types";
import { CardBack } from "../components/CardBack";
import { PlayingCard } from "../components/PlayingCard";
import { preloadFaces } from "../lib/media";
import { useElementSize, type Size } from "../lib/useElementSize";
import { FocusCard } from "./FocusCard";

type Phase = "loading" | "idle" | "shuffling" | "dealing" | "dealt" | "collecting" | "discarding";

const DRAW_COUNT = 3;
const STACK_DEPTH = 7;
const DISCARD_DEPTH = 4;
const SHUFFLE_MS = 1500;
const DEAL_STAGGER = 0.22;
const COLLECT_MS = 650;
const DISCARD_MS = 700;
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
  const discardAside: Placement = { x: width / 2 + deckW * smallScale + gap, y: deckAside.y, scale: smallScale };
  const handY = top + deckSmallH + rowGap + handH / 2;
  const slot = (i: number): Placement => ({ x: width / 2 + (i - (n - 1) / 2) * (handW + gap), y: handY, scale: 1 });
  return { handW, handH, deckW, deckIdle, deckAside, discardAside, slot };
}

function revealedKey(deckID: string): string {
  return `${REVEALED_KEY}.${deckID}`;
}

function loadRevealed(deckID: string, drawID: string): Set<string> {
  try {
    const saved = JSON.parse(localStorage.getItem(revealedKey(deckID)) ?? "null") as { drawID: string; cardIDs: string[] } | null;
    if (saved?.drawID === drawID) return new Set(saved.cardIDs);
  } catch {
    // A corrupt entry just means nothing is revealed.
  }
  return new Set();
}

function saveRevealed(deckID: string, drawID: string, revealed: Set<string>) {
  localStorage.setItem(revealedKey(deckID), JSON.stringify({ drawID, cardIDs: [...revealed] }));
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

interface DrawViewProps {
  api: DeckApi;
  deck: Deck;
  onDeckChanged: () => void;
  onManage: () => void;
}

export function DrawView({ api, deck, onDeckChanged, onManage }: DrawViewProps) {
  const stageRef = useRef<HTMLDivElement>(null);
  const size = useElementSize(stageRef);
  const [phase, setPhase] = useState<Phase>("loading");
  const [draw, setDraw] = useState<Draw | null>(null);
  const [discard, setDiscard] = useState<DealtCard[]>([]);
  const [remaining, setRemaining] = useState(0);
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [focusIndex, setFocusIndex] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPhase("loading");
    setDraw(null);
    setDiscard([]);
    setFocusIndex(null);
    setError(null);
    api
      .currentDraw(deck.id)
      .then(async (current) => {
        if (cancelled) return;
        if (current && current.cards.length > 0) {
          await preloadFaces([...current.cards, ...current.discard.slice(-1)], 3000);
          if (cancelled) return;
          setDraw(current);
          setDiscard(current.discard);
          setRemaining(current.remaining);
          setRevealed(loadRevealed(deck.id, current.id));
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
  const handOut = phase === "dealing" || phase === "dealt" || phase === "collecting" || phase === "discarding";
  const busy = phase === "loading" || phase === "shuffling" || phase === "dealing" || phase === "collecting" || phase === "discarding";
  const canDraw = phase === "dealt" && remaining > 0;
  const deckPlacement = handOut ? layout.deckAside : layout.deckIdle;
  const deckAsHand: Placement = { ...deckPlacement, scale: (layout.deckW * deckPlacement.scale) / layout.handW };
  const discardAsHand: Placement = { ...layout.discardAside, scale: (layout.deckW * layout.discardAside.scale) / layout.handW };
  // The pile thins as the pass runs down and fills again once cards are gathered for a shuffle.
  const deckDepth = handOut && phase !== "collecting" ? Math.min(STACK_DEPTH, remaining) : STACK_DEPTH;

  const fetchDraw = useCallback(
    async (shuffle: boolean) => {
      const dealt = await api.draw(deck.id, DRAW_COUNT, shuffle);
      await preloadFaces(dealt.cards);
      return dealt;
    },
    [api, deck.id],
  );

  const deal = useCallback(
    async (dealt: Draw) => {
      setDraw(dealt);
      setRemaining(dealt.remaining);
      setRevealed(new Set());
      saveRevealed(deck.id, dealt.id, new Set());
      setPhase("dealing");
      onDeckChanged();
      await sleep(DEAL_STAGGER * 1000 * (dealt.cards.length - 1) + 700);
      setPhase("dealt");
    },
    [deck.id, onDeckChanged],
  );

  /** Gathers the hand and the discard back into the deck, shuffles, and deals from a fresh pass. */
  const shuffleAndDraw = useCallback(async () => {
    if (busy) return;
    setError(null);
    setFocusIndex(null);
    if (draw) {
      setPhase("collecting");
      await sleep(COLLECT_MS + DEAL_STAGGER * 1000 * (draw.cards.length - 1));
      setDraw(null);
    }
    setDiscard([]);
    setPhase("shuffling");
    try {
      const [dealt] = await Promise.all([fetchDraw(true), sleep(SHUFFLE_MS)]);
      await deal(dealt);
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        setError("empty");
      } else {
        setError(err instanceof Error ? err.message : "Something went wrong");
      }
      setPhase("idle");
    }
  }, [busy, deal, draw, fetchDraw]);

  /** Lays the hand on the discard pile and deals the next cards in the pass without shuffling. */
  const drawNext = useCallback(async () => {
    if (!canDraw || !draw) return;
    setError(null);
    setFocusIndex(null);
    setPhase("discarding");
    try {
      const [dealt] = await Promise.all([fetchDraw(false), sleep(DISCARD_MS + DEAL_STAGGER * 1000 * (draw.cards.length - 1))]);
      setDiscard((prev) => [...prev, ...draw.cards]);
      await deal(dealt);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
      setPhase("dealt");
    }
  }, [canDraw, deal, draw, fetchDraw]);

  const reveal = useCallback(
    (cardIDs: string[]) => {
      if (!draw) return;
      setRevealed((prev) => {
        const next = new Set(prev);
        cardIDs.forEach((id) => next.add(id));
        saveRevealed(deck.id, draw.id, next);
        return next;
      });
    },
    [deck.id, draw],
  );


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
                className={`deck-button${phase === "idle" ? " deck-ready" : ""}${deckDepth === 0 ? " deck-empty" : ""}`}
                onClick={phase === "idle" ? shuffleAndDraw : drawNext}
                disabled={phase !== "idle" && !canDraw}
                aria-label={phase === "idle" ? "Shuffle and draw three cards" : "Draw the next cards"}
              >
                {Array.from({ length: deckDepth }, (_, i) => (
                  <ShuffleCard key={i} index={i} width={layout.deckW} shuffling={phase === "shuffling"} />
                ))}
              </button>
            </motion.div>

            {discard.length > 0 && (
              <motion.div
                className="discard"
                style={{ width: layout.deckW, height: layout.deckW * 1.5, left: -layout.deckW / 2, top: (-layout.deckW * 1.5) / 2 }}
                initial={false}
                animate={{ ...(phase === "collecting" ? deckPlacement : layout.discardAside) }}
                transition={{ type: "spring", stiffness: 150, damping: 22 }}
                aria-label={`${discard.length} ${discard.length === 1 ? "card" : "cards"} drawn this pass`}
              >
                {Array.from({ length: Math.min(discard.length - 1, DISCARD_DEPTH) }, (_, i) => (
                  <div key={i} className="stack-card" style={{ transform: `translateY(${(DISCARD_DEPTH - i) * 1.4}px)` }}>
                    <CardBack />
                  </div>
                ))}
                <div className="discard-top">
                  <PlayingCard card={discard[discard.length - 1]} width={layout.deckW} faceUp={phase !== "collecting"} tactile={false} />
                </div>
              </motion.div>
            )}

            {draw &&
                draw.cards.map((card, i) => {
                  const target = phase === "collecting" ? deckAsHand : phase === "discarding" ? discardAsHand : layout.slot(i);
                  const order = phase === "collecting" ? draw.cards.length - 1 - i : i;
                  const gathered = phase === "collecting" || phase === "discarding";
                  return (
                    <motion.div
                      key={`${draw.id}-${card.id}`}
                      className="hand-card"
                      style={{ width: layout.handW, left: -layout.handW / 2, top: -layout.handH / 2, zIndex: 10 + i }}
                      initial={{ ...deckAsHand, rotate: -4 + i * 3 }}
                      animate={{ ...target, rotate: gathered ? 0 : (i - (draw.cards.length - 1) / 2) * 1.5 }}
                      transition={{ ...dealSpring, delay: phase === "dealt" ? 0 : order * DEAL_STAGGER }}
                    >
                      <PlayingCard
                        card={card}
                        width={layout.handW}
                        faceUp={phase === "discarding" || (revealed.has(card.id) && phase !== "collecting")}
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
            key={error === "empty" ? "empty" : phase}
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
              <button type="button" className="btn btn-primary" onClick={shuffleAndDraw}>
                Shuffle &amp; draw
              </button>
            ) : phase === "shuffling" ? (
              <p>Shuffling…</p>
            ) : phase === "dealt" ? (
              <div className="draw-actions">
                <button type="button" className="btn" onClick={shuffleAndDraw}>
                  Shuffle
                </button>
                <button type="button" className="btn" onClick={drawNext} disabled={!canDraw}>
                  Draw
                </button>
              </div>
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
            <FocusCard api={api} card={draw.cards[focusIndex]} onClose={() => setFocusIndex(null)} />
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

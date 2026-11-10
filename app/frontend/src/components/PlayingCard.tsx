import { motion, useMotionValue, useSpring, useTransform } from "motion/react";
import type { PointerEvent } from "react";
import type { Card } from "../api/types";
import { CardBack } from "./CardBack";
import { CardFace } from "./CardFace";

interface PlayingCardProps {
  card?: Card;
  faceUp: boolean;
  width: number;
  /** Tilts toward the pointer and lifts on hover or press. */
  tactile?: boolean;
  onActivate?: () => void;
  label?: string;
}

const flipSpring = { type: "spring", stiffness: 140, damping: 18, mass: 0.9 } as const;
const tiltSpring = { stiffness: 220, damping: 20 };

/** A two-sided card that flips in 3D and responds to the pointer like something in the hand. */
export function PlayingCard({ card, faceUp, width, tactile = true, onActivate, label }: PlayingCardProps) {
  const px = useMotionValue(0);
  const py = useMotionValue(0);
  const rotateX = useSpring(useTransform(py, [-0.5, 0.5], [9, -9]), tiltSpring);
  const rotateY = useSpring(useTransform(px, [-0.5, 0.5], [-11, 11]), tiltSpring);
  const glareX = useTransform(px, [-0.5, 0.5], ["20%", "80%"]);
  const glareY = useTransform(py, [-0.5, 0.5], ["15%", "85%"]);

  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!tactile || e.pointerType !== "mouse") return;
    const rect = e.currentTarget.getBoundingClientRect();
    px.set((e.clientX - rect.left) / rect.width - 0.5);
    py.set((e.clientY - rect.top) / rect.height - 0.5);
  };
  const onPointerLeave = () => {
    px.set(0);
    py.set(0);
  };

  return (
    <motion.div
      className={`pcard${onActivate ? " pcard-clickable" : ""}`}
      style={{ width, height: width * 1.5, rotateX, rotateY }}
      onPointerMove={onPointerMove}
      onPointerLeave={onPointerLeave}
      whileHover={tactile ? { scale: 1.04, z: 30 } : undefined}
      whileTap={tactile ? { scale: 0.98 } : undefined}
      onClick={onActivate}
      role={onActivate ? "button" : undefined}
      tabIndex={onActivate ? 0 : undefined}
      aria-label={label}
      onKeyDown={(e) => {
        if (onActivate && (e.key === "Enter" || e.key === " ")) {
          e.preventDefault();
          onActivate();
        }
      }}
    >
      <motion.div
        className="pcard-inner"
        initial={false}
        animate={{ rotateY: faceUp ? 0 : 180 }}
        transition={flipSpring}
      >
        <div className="pcard-side pcard-front">
          {card ? <CardFace card={card} /> : <CardBack />}
          {tactile && <motion.div className="pcard-glare" style={{ "--gx": glareX, "--gy": glareY } as never} />}
        </div>
        <div className="pcard-side pcard-back">
          <CardBack />
        </div>
      </motion.div>
    </motion.div>
  );
}

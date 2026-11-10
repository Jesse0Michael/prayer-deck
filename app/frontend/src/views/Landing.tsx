import { motion } from "motion/react";
import { useState } from "react";
import { DemoApi } from "../api/demoApi";
import { CardBack } from "../components/CardBack";
import { DeckApp } from "./DeckApp";

const CONTACT_EMAIL = "jesse.michael1234@gmail.com";
const CONTACT_HREF = `mailto:${CONTACT_EMAIL}?subject=${encodeURIComponent("Prayer Deck feedback")}`;

interface LandingProps {
  signedIn: boolean;
}

/** The public front page: what Prayer Deck is, a working demo, and a way to reach its author. */
export function Landing({ signedIn }: LandingProps) {
  const cta = signedIn ? { href: "#/", label: "Open your deck" } : { href: "#/signin", label: "Start your deck" };

  return (
    <div className="landing">
      <header className="landing-header">
        <a className="hand-title brand" href={signedIn ? "#/" : "#/about"}>
          Prayer Deck
        </a>
        <span className="spacer" />
        <a className="btn btn-primary" href={cta.href}>
          {signedIn ? "My deck" : "Sign in"}
        </a>
      </header>

      <Hero ctaHref={cta.href} ctaLabel={cta.label} />
      <HowItWorks />
      <Demo />
      <Contact />
    </div>
  );
}

function Hero({ ctaHref, ctaLabel }: { ctaHref: string; ctaLabel: string }) {
  return (
    <section className="landing-hero">
      <div className="signin-fan" aria-hidden="true">
        {[-14, 0, 14].map((rotate, i) => (
          <motion.div
            key={rotate}
            className="signin-fan-card"
            initial={{ y: 60, rotate: 0, opacity: 0 }}
            animate={{ y: Math.abs(rotate) / 2, rotate, opacity: 1 }}
            transition={{ type: "spring", stiffness: 90, damping: 14, delay: 0.1 + i * 0.12 }}
          >
            <CardBack />
          </motion.div>
        ))}
      </div>
      <h1 className="hand-title">Prayer Deck</h1>
      <p className="landing-lede">
        A deck of illustrated cards for the people you love. Shuffle, draw three, and spend a few quiet minutes praying
        for whoever turns up.
      </p>
      <div className="landing-actions">
        <a className="btn btn-primary" href={ctaHref}>
          {ctaLabel}
        </a>
        <button
          type="button"
          className="btn"
          onClick={() => document.getElementById("demo")?.scrollIntoView({ behavior: "smooth", block: "start" })}
        >
          Try the demo
        </button>
      </div>
    </section>
  );
}

function HowItWorks() {
  const steps = [
    { title: "Shuffle", body: "Your deck holds a card for each friend, relative, couple, or family you want to pray for." },
    { title: "Draw three", body: "Cards are dealt face down. Turn them over one at a time, with nothing to rush." },
    { title: "Pray", body: "Open a card to see the picture, read your notes, and spend as long as you like." },
  ];
  return (
    <section className="landing-section">
      <h2 className="hand-title">How it works</h2>
      <ol className="landing-steps">
        {steps.map((s, i) => (
          <li key={s.title}>
            <span className="landing-step-number hand-title">{i + 1}</span>
            <h3>{s.title}</h3>
            <p>{s.body}</p>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Demo() {
  const [demo, setDemo] = useState(() => ({ api: new DemoApi(), round: 0 }));
  const reset = () => setDemo((prev) => ({ api: new DemoApi(), round: prev.round + 1 }));

  return (
    <section className="landing-section landing-demo" id="demo">
      <h2 className="hand-title">Try it</h2>
      <p className="landing-section-lede">
        This is the real app with a pretend deck. Shuffle and draw, open a card to read its notes, or switch to Cards to
        add people and pictures. Nothing leaves your browser, and it all resets when you leave.
      </p>
      <div className="landing-demo-frame">
        <DeckApp
          key={demo.round}
          api={demo.api}
          embedded
          actions={
            <button type="button" className="btn btn-quiet" onClick={reset}>
              Reset
            </button>
          }
        />
      </div>
    </section>
  );
}

function Contact() {
  return (
    <section className="landing-section landing-contact" id="contact">
      <h2 className="hand-title">Tell me what you think</h2>
      <a className="btn btn-primary" href={CONTACT_HREF}>
        Send feedback
      </a>
    </section>
  );
}

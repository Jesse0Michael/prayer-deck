import "@fontsource/caveat/500.css";
import "@fontsource/caveat/700.css";
import "@fontsource/lora/400.css";
import "@fontsource/lora/500.css";
import "@fontsource/lora/400-italic.css";
import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import cardBackUrl from "./assets/card-back.webp";
import { preloadImage } from "./lib/media";
import "./styles.css";

// Every deal starts face down, so the back must be decoded before the first card appears.
preloadImage(cardBackUrl);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);

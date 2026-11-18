import { createAnalytics } from "./googleTag";
export type { DeckMode } from "./googleTag";

export const { trackPage, trackLogin, trackHandDealt, trackCard } = createAnalytics({
  measurementId: import.meta.env.VITE_GA_MEASUREMENT_ID,
  hostname: import.meta.env.VITE_ANALYTICS_HOSTNAME,
  production: import.meta.env.PROD,
  debugRequested: import.meta.env.VITE_ANALYTICS_DEBUG === "true",
}, window);

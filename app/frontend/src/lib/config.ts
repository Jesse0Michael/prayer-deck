import { ConfigService } from "../../bindings/github.com/jesse0michael/prayer-deck/app";

const BUILD_API_URL: string | undefined = import.meta.env.VITE_API_URL;
const FALLBACK_API_URL = BUILD_API_URL ?? "http://localhost:8080";

/**
 * Loads the client configuration from the Wails Go service, falling back to
 * the build-time default when the page runs outside Wails (plain `vite dev`).
 * A build with VITE_API_URL set is a static deployment with no Wails server,
 * so it uses that URL without waiting on the service.
 */
export async function loadAPIURL(): Promise<string> {
  if (BUILD_API_URL) return BUILD_API_URL.replace(/\/$/, "");
  try {
    const cfg = await Promise.race([
      ConfigService.Config(),
      new Promise<never>((_, reject) => setTimeout(() => reject(new Error("config timeout")), 2500)),
    ]);
    if (cfg.apiURL) return cfg.apiURL.replace(/\/$/, "");
  } catch (err) {
    console.warn("Using fallback API URL; Wails config unavailable", err);
  }
  return FALLBACK_API_URL;
}

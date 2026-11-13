import { useEffect, useState } from "react";

/** True once `active` has held for `ms`, so slow waits get a note that quick ones never flash. */
export function useDelayed(active: boolean, ms: number): boolean {
  const [elapsed, setElapsed] = useState(false);
  useEffect(() => {
    setElapsed(false);
    if (!active) return;
    const timer = setTimeout(() => setElapsed(true), ms);
    return () => clearTimeout(timer);
  }, [active, ms]);
  return active && elapsed;
}

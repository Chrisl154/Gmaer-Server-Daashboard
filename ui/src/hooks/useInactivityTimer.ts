import { useEffect, useRef } from 'react';

// 29 minutes 50 seconds — matches the server-side 2h token with room to spare
// for the inactivity logout to fire well before the token would naturally expire.
const INACTIVITY_MS = 29 * 60 * 1000 + 50 * 1000; // 1_790_000 ms

const ACTIVITY_EVENTS = [
  'mousemove',
  'mousedown',
  'keydown',
  'scroll',
  'touchstart',
  'click',
] as const;

/**
 * Calls `onTimeout` after INACTIVITY_MS of no user activity.
 * The timer resets on any mouse, keyboard, scroll, or touch event.
 * Cleans up all listeners and the timer on unmount.
 */
export function useInactivityTimer(onTimeout: () => void): void {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Keep a stable ref to onTimeout so the effect doesn't re-run when the
  // callback identity changes between renders.
  const callbackRef = useRef(onTimeout);
  callbackRef.current = onTimeout;

  useEffect(() => {
    const reset = () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => callbackRef.current(), INACTIVITY_MS);
    };

    reset(); // start immediately on mount
    ACTIVITY_EVENTS.forEach(evt =>
      window.addEventListener(evt, reset, { passive: true })
    );

    return () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current);
      ACTIVITY_EVENTS.forEach(evt => window.removeEventListener(evt, reset));
    };
  }, []); // empty deps — effect runs once, callback changes handled via ref
}

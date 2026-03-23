import { useEffect, useRef, useState } from 'react';
import { api } from '../utils/api';

/**
 * Monitors daemon connectivity. Returns `connected` (boolean) which flips to
 * false after 2 consecutive failed health checks and back to true on success.
 * Checks run every 3 s while disconnected and every 15 s while connected.
 */
export function useConnectionStatus() {
  const [connected, setConnected] = useState(true);
  const failCount = useRef(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    const check = async () => {
      try {
        await api.get('/healthz', { timeout: 5_000 });
        failCount.current = 0;
        setConnected(true);
      } catch (err: unknown) {
        // If the daemon responded with any HTTP status (even 401/404), it's
        // reachable — only network-level failures (timeout, ECONNREFUSED)
        // mean the daemon is actually down.
        const axErr = err as { response?: { status: number } };
        if (axErr?.response?.status) {
          failCount.current = 0;
          setConnected(true);
        } else {
          failCount.current++;
          if (failCount.current >= 2) setConnected(false);
        }
      }
    };

    // Start polling — fast while disconnected, slow while connected.
    const start = () => {
      if (timerRef.current) clearInterval(timerRef.current);
      timerRef.current = setInterval(check, connected ? 15_000 : 3_000);
    };

    start();
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, [connected]);

  return connected;
}

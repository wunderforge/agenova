// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';
import { connectedSource, isTerminal, type Setup, type View } from './connected-source';

// Polling stops after the final outcome, not merely the terminal claim phase:
// cleanup can still be in progress after authority is revoked.
export function useConnection(parts: string[], revision: number) {
  const [setup, setSetup] = useState<Setup>();
  const [works, setWorks] = useState<View[]>([]);
  const [current, setCurrent] = useState<View>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [setupError, setSetupError] = useState('');
  const [paused, setPaused] = useState(false);
  let ref = '';
  let routeError = '';
  try {
    // Consume the decoded array: production optimizers can remove an unused
    // decode call, including its validation side effect.
    const decodedParts = parts.map(part => decodeURIComponent(part));
    if (decodedParts[0] === 'work' && decodedParts[1] && decodedParts[1] !== 'new') ref = decodedParts[1];
  } catch {
    routeError = 'Invalid work reference.';
  }

  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;
    let setupLoaded = false;
    const started = Date.now();
    setLoading(true); setError(''); setCurrent(undefined); setPaused(false);
    if (routeError) {
      setLoading(false); setError(routeError);
      return () => controller.abort();
    }

    async function load() {
      try {
        const [nextSetup, list, detail] = await Promise.all([
          // Setup may query the installed Platform and Kubernetes. Load once
          // per connection/route revision; only Work evidence is polled.
          setupLoaded ? Promise.resolve(undefined) : connectedSource.setup(controller.signal),
          connectedSource.list(controller.signal),
          ref ? connectedSource.request(ref, controller.signal) : Promise.resolve(undefined),
        ]);
        if (disposed) return;
        if (nextSetup) { setSetup(nextSetup); setSetupError(''); setupLoaded = true; }
        setWorks(list); setCurrent(detail);
        setLoading(false); setError('');
        if (detail && isTerminal(detail) && detail.outcome) return;
        if (Date.now() - started >= 120_000) { setPaused(true); return; }
        timer = setTimeout(load, 1000);
      } catch (cause) {
        if (disposed) return;
        setLoading(false);
        setError(cause instanceof Error ? cause.message : 'The connection is unavailable.');
      }
    }
    void load();
    return () => {
      disposed = true; controller.abort();
      if (timer) clearTimeout(timer);
    };
  }, [ref, routeError, revision]);

  // Registry-backed setup is much more expensive than evidence polling, but
  // it must not remain indefinitely stale on a terminal Work or idle page.
  useEffect(() => {
    const controller = new AbortController();
    let disposed = false;
    let refreshing = false;
    const timer = setInterval(async () => {
      if (refreshing) return;
      refreshing = true;
      try {
        const next = await connectedSource.setup(controller.signal);
        if (!disposed) { setSetup(next); setSetupError(''); }
      } catch (cause) {
        if (!disposed) {
          setSetup(undefined);
          setSetupError(cause instanceof Error ? cause.message : 'Platform setup is unavailable.');
        }
      } finally {
        refreshing = false;
      }
    }, 30_000);
    return () => { disposed = true; clearInterval(timer); controller.abort(); };
  }, [revision]);

  // Guard synchronously: hash navigation can render a new record route with
  // the previous work's state before the effect has reset it.
  return { setup, works, current, loading: routeError ? false : loading, error: routeError || setupError || error, paused };
}


// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef } from 'react';

// Native hover/press feedback; no pointer tracking, moving gradients or page fades.
export function usePortalMotion(routeKey: string) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const bar = root.current?.querySelector<HTMLElement>('.portal-scroll-progress');
    if (!bar) return;
    let frame = 0;
    let extent = 0;
    const scroll = () => {
      if (frame) return;
      frame = requestAnimationFrame(() => {
        bar.style.setProperty('--scroll-progress', String(extent > 0 ? Math.min(1, scrollY / extent) : 0));
        frame = 0;
      });
    };
    const measure = () => { extent = document.documentElement.scrollHeight - innerHeight; scroll(); };
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure);
    observer?.observe(document.body);
    window.addEventListener('scroll', scroll, { passive: true });
    window.addEventListener('resize', measure, { passive: true });
    measure();
    return () => {
      observer?.disconnect(); cancelAnimationFrame(frame);
      window.removeEventListener('scroll', scroll); window.removeEventListener('resize', measure);
    };
  }, []);
  useEffect(() => { window.scrollTo({ top: 0, behavior: 'instant' }); }, [routeKey]);
  return root;
}

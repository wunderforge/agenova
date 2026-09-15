// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef } from 'react';

const surfaces = '.portal-flow,.portal-panel,.portal-table-wrap,.portal-access-card,.portal-record-list,.portal-result';
const controls = '.portal-button,.portal-filters button,.portal-sidebar nav a,.portal-mode-switch button,.portal-identity';
export function usePortalMotion(routeKey: string) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const shell = root.current;
    if (!shell) return;
    const quiet = matchMedia('(prefers-reduced-motion: reduce)');
    let pointerFrame = 0, scrollFrame = 0;
    let lit: HTMLElement | null = null;
    const animations = new Set<Animation>();
    const animate = (element: Element, frames: Keyframe[], options: KeyframeAnimationOptions) => {
      if (quiet.matches || !element.animate) return;
      const motion = element.animate(frames, options);
      animations.add(motion);
      void motion.finished.then(() => animations.delete(motion), () => animations.delete(motion));
    };
    const scroll = () => {
      if (scrollFrame) return;
      scrollFrame = requestAnimationFrame(() => {
        const max = document.documentElement.scrollHeight - innerHeight;
        shell.querySelector<HTMLElement>('.portal-scroll-progress')?.style.setProperty('--scroll-progress', String(max > 0 ? Math.min(1, scrollY / max) : 0));
        shell.querySelector('.portal-topbar')?.classList.toggle('is-scrolled', scrollY > 12);
        scrollFrame = 0;
      });
    };
    const pointer = (event: PointerEvent) => {
      if (quiet.matches || event.pointerType !== 'mouse') return;
      const box = (event.target as Element).closest<HTMLElement>(surfaces);
      if (lit !== box) lit?.classList.remove('pointer-lit');
      lit = box;
      cancelAnimationFrame(pointerFrame);
      if (!box) return;
      pointerFrame = requestAnimationFrame(() => {
        if (lit !== box || !box.isConnected) return;
        const rect = box.getBoundingClientRect();
        box.style.setProperty('--pointer-x', event.clientX - rect.left + 'px');
        box.style.setProperty('--pointer-y', event.clientY - rect.top + 'px');
        box.classList.add('pointer-lit');
      });
    };
    const leave = () => { lit?.classList.remove('pointer-lit'); lit = null; };
    const click = (event: MouseEvent) => {
      const button = (event.target as Element).closest<HTMLElement>(controls);
      if (button) animate(button, [{ boxShadow: 'inset 0 0 28px #92b7ff55' }, { boxShadow: 'inset 0 0 0 #92b7ff00' }], { duration: 650, easing: 'ease-out' });
    };
    const toggle = (event: Event) => {
      if (event.target instanceof HTMLDetailsElement && event.target.open)
        animate(event.target, [{ opacity: .55 }, { opacity: 1 }], { duration: 450, easing: 'ease-out' });
    };
    const stop = () => { if (quiet.matches) { animations.forEach(a => a.cancel()); leave(); } };
    shell.addEventListener('pointermove', pointer, { passive: true });
    shell.addEventListener('pointerleave', leave);
    shell.addEventListener('click', click);
    shell.addEventListener('toggle', toggle, true);
    window.addEventListener('scroll', scroll, { passive: true });
    window.addEventListener('resize', scroll, { passive: true });
    quiet.addEventListener('change', stop);
    scroll();
    return () => {
      cancelAnimationFrame(pointerFrame); cancelAnimationFrame(scrollFrame);
      animations.forEach(a => a.cancel()); leave();
      shell.removeEventListener('pointermove', pointer); shell.removeEventListener('pointerleave', leave);
      shell.removeEventListener('click', click); shell.removeEventListener('toggle', toggle, true);
      window.removeEventListener('scroll', scroll); window.removeEventListener('resize', scroll);
      quiet.removeEventListener('change', stop);
    };
  }, []);
  useEffect(() => {
    const content = root.current?.querySelector('main');
    if (!content) return;
    const quiet = matchMedia('(prefers-reduced-motion: reduce)').matches;
    window.scrollTo({ top: 0, behavior: quiet ? 'instant' : 'smooth' });
    const animation = !quiet && content.animate?.([{ opacity: .82 }, { opacity: 1 }], { duration: 340, easing: 'ease-out' });
    return () => { if (animation) animation.cancel(); };
  }, [routeKey]);
  return root;
}

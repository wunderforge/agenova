// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { chromium } from 'playwright';
import { mkdir, writeFile } from 'node:fs/promises';
const label = process.argv[2] || 'current';
if (!/^[a-z0-9-]+$/.test(label)) throw Error('Use a simple report label.');
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Performance.enable');
  const reports = [];
  for (const scenario of ['pointer', 'scroll']) {
    await page.goto('http://127.0.0.1:5175/' + (scenario === 'scroll' ? '#/work/fix-payment-timeout' : '#/work'));
    await page.locator('.portal-table-wrap,.portal-flow').first().waitFor();
    await page.waitForTimeout(600);
    const before = (await cdp.send('Performance.getMetrics')).metrics;
    const trace = { Paint: { count: 0, ms: 0 }, Layout: { count: 0, ms: 0 }, UpdateLayoutTree: { count: 0, ms: 0 } };
    const collect = ({ value }) => value.forEach(e => {
      if (e.ph === 'X' && trace[e.name]) { trace[e.name].count++; trace[e.name].ms += (e.dur || 0) / 1000; }
    });
    cdp.on('Tracing.dataCollected', collect);
    await cdp.send('Tracing.start', { categories: 'devtools.timeline', transferMode: 'ReportEvents' });
    const timing = await page.evaluate(async scenario => {
      const box = document.querySelector('.portal-table-wrap,.portal-access-card');
      const times = []; let start = 0, previous = 0;
      await new Promise(resolve => {
        function frame(now) {
          if (!start) start = now;
          if (previous) times.push(now - previous);
          previous = now;
          const phase = (now - start) / 3000;
          if (scenario === 'pointer') {
            const r = box.getBoundingClientRect();
            box.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerType: 'mouse', clientX: r.left + r.width * (.5 + .4 * Math.sin(phase * 14)), clientY: r.top + r.height * (.5 + .4 * Math.cos(phase * 11)) }));
          } else window.scrollTo({ top: 230 * (1 - Math.cos(phase * Math.PI * 4)), behavior: 'instant' });
          if (phase < 1) requestAnimationFrame(frame); else resolve();
        }
        requestAnimationFrame(frame);
      });
      times.sort((a,b) => a-b);
      return { frames: times.length, p95FrameMs: times[Math.floor(times.length * .95)], framesOver25ms: times.filter(t => t > 25).length };
    }, scenario);
    const completed = new Promise(resolve => cdp.once('Tracing.tracingComplete', resolve));
    await cdp.send('Tracing.end'); await completed;
    cdp.off('Tracing.dataCollected', collect);
    const after = (await cdp.send('Performance.getMetrics')).metrics;
    const cpu = Object.fromEntries(['TaskDuration','ScriptDuration','LayoutDuration','RecalcStyleDuration'].map(name => [name + 'Ms', Math.round(((after.find(m => m.name === name)?.value || 0) - (before.find(m => m.name === name)?.value || 0)) * 1000)]));
    reports.push({ scenario, ...timing, cpu, trace });
  }
  const dir = new URL('../../.tmp/ui-motion-performance/', import.meta.url);
  await mkdir(dir, { recursive: true });
  const report = { label, environment: 'Headless Chromium, 1440x1000, 3-second fixed workloads; relative diagnostic, not a user-device FPS guarantee', reports };
  await writeFile(new URL(label + '.json', dir), JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report));
} finally { await browser.close(); }

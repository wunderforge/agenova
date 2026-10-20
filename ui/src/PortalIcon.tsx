// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
const paths = {
  work: 'M2 10h4l2-6 4 12 2-6h6',
  agents: 'M8 3h6M11 1v2M5 7h12v11H5zM2 10v5M20 10v5M8 11h.01M14 11h.01M8 15h6',
  policy: 'M11 2l8 3v6c0 5-8 9-8 9S3 16 3 11V5zM7 10l3 3 5-6',
  platform: 'M7 3h8v5H7zM3 14h6v5H3zM13 14h6v5h-6zM11 8v3M6 14v-3h10v3',
} as const;
export function PortalIcon({ name }: { name: keyof typeof paths }) {
  return <svg className="portal-nav-icon" width="18" height="18" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.35" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[name]}/></svg>;
}

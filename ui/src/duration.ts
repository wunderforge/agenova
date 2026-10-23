// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Templates carry Go duration strings. Never read "30s" as thirty minutes.
export function requestedMinutes(duration?: string | null): number {
 const factors:Record<string,number>={ns:1e-9,us:1e-6,'µs':1e-6,ms:1e-3,s:1,m:60,h:3600};
 let consumed='',seconds=0;
 for(const match of (duration || '').matchAll(/(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g)){
  consumed+=match[0];seconds+=Number(match[1])*factors[match[2]];
 }
 return consumed===duration && seconds>0 ? Math.min(20,seconds/60) : 20;
}

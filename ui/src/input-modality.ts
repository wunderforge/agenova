// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
export function installInputModality(doc:Document){
 doc.addEventListener('pointerdown',()=>{doc.documentElement.dataset.inputModality='pointer';},true);
 doc.addEventListener('keydown',event=>{if(event.key==='Tab')doc.documentElement.dataset.inputModality='keyboard';},true);
}

// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Public product preview only. Sign-in links use the existing server login route.
const imageDialog=document.querySelector('#image-dialog');
const previewImage=document.querySelector('#product-image');
const previews={work:{src:'/landing/assets/work-explorer.png',alt:'Actual Agenova React UI: Work list showing separate admission decisions and execution outcomes',description:'Admission decisions and execution outcomes stay separate.'},evidence:{src:'/landing/assets/work-evidence.png',alt:'Actual Agenova React UI: recorded activity, selected evidence inspector and access limits',description:'Follow recorded activity. Inspect the evidence behind each call.'}};
function selectPreview(button){
 const key=button.dataset.preview,preview=previews[key];
 document.querySelectorAll('[data-preview]').forEach(tab=>{const active=tab===button;tab.setAttribute('aria-selected',String(active));tab.tabIndex=active?0:-1;});
 previewImage.src=preview.src;previewImage.alt=preview.alt;
 document.querySelector('#preview-panel').setAttribute('aria-labelledby',button.id);
 document.querySelector('#preview-description').textContent=preview.description;
 document.querySelector('#screenshot-button').setAttribute('aria-label',`Enlarge ${key==='work'?'Work explorer':'Work evidence'} screenshot`);
}
const tabs=[...document.querySelectorAll('[data-preview]')];
tabs.forEach((button,index)=>{button.addEventListener('click',()=>selectPreview(button));button.addEventListener('keydown',event=>{let next;if(event.key==='ArrowRight')next=(index+1)%tabs.length;if(event.key==='ArrowLeft')next=(index+tabs.length-1)%tabs.length;if(event.key==='Home')next=0;if(event.key==='End')next=tabs.length-1;if(next!==undefined){event.preventDefault();selectPreview(tabs[next]);tabs[next].focus();}});});
function enlarge(){const image=document.querySelector('#enlarged-image');image.src=previewImage.src;image.alt=previewImage.alt;imageDialog.showModal();}
document.querySelector('#expand-preview').addEventListener('click',enlarge);
document.querySelector('#screenshot-button').addEventListener('click',enlarge);
document.querySelector('.image-dismiss').addEventListener('click',()=>imageDialog.close());
for(const dialog of [imageDialog])dialog.addEventListener('click',event=>{if(event.target!==dialog)return;const r=dialog.getBoundingClientRect();if(event.clientX<r.left||event.clientX>r.right||event.clientY<r.top||event.clientY>r.bottom)dialog.close();});

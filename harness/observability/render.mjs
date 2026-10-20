// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Captures only synthetic reference Work and dashboard views, never login forms.
import {execFileSync,spawn} from 'node:child_process';
import {randomBytes} from 'node:crypto';
import {readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {createRequire} from 'node:module';
const i=process.argv.indexOf('--context'),context=process.argv[i+1];if(i<0||!context)throw new Error('Explicit --context required');
const namespace='agenova-observability-180',directory=resolve('.tmp/observability180');
const result=JSON.parse(readFileSync(resolve(directory,'result.json'),'utf8'));
let faults;try{faults=JSON.parse(readFileSync(resolve(directory,'fault-result.json'),'utf8'))}catch{}
const password=randomBytes(24).toString('base64url');
// Rotate only the dedicated test admin, keeping Secret and database consistent.
execFileSync('kubectl',['--context',context,'-n',namespace,'apply','-f','-'],{input:JSON.stringify({apiVersion:'v1',kind:'Secret',metadata:{name:'grafana-admin',namespace,labels:{'app.kubernetes.io/managed-by':'agenova'}},type:'Opaque',stringData:{username:'admin',password}}),stdio:['pipe','ignore','inherit']});
execFileSync('kubectl',['--context',context,'-n',namespace,'exec','-i','deployment/agenova-grafana','--','grafana','cli','--homepath','/usr/share/grafana','admin','reset-admin-password','--password-from-stdin'],{input:password+'\n',stdio:['pipe','ignore','inherit']});
const children=[];
function forward(resource,mapping){children.push(spawn('kubectl',['--context',context,'-n',namespace,'port-forward',resource,mapping,'--address','127.0.0.1'],{stdio:'ignore'}))}
forward('deployment/agenova-control-plane','8098:8081');forward('service/agenova-grafana','3000:3000');
children.push(spawn(process.execPath,[resolve('ui/node_modules/vite/bin/vite.js'),'--host','127.0.0.1','--port','5177','--strictPort'],{cwd:resolve('ui'),env:{...process.env,AGENOVA_API_URL:'http://127.0.0.1:8098'},stdio:'ignore'}));
const delay=ms=>new Promise(r=>setTimeout(r,ms));const require=createRequire(resolve('ui/package.json'));const{chromium,expect}=require('@playwright/test');
let browser;
try{
 for(let n=0;n<60;n++){try{if((await fetch('http://127.0.0.1:5177/api/setup')).ok)break}catch{}await delay(1000)}
 browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:1600,height:1000}});
 await page.goto('http://127.0.0.1:3000/login');await page.getByRole('textbox',{name:/email or username/i}).fill('admin');await page.getByLabel('Password',{exact:true}).fill(password);await page.getByRole('button',{name:/log in/i}).click();await page.waitForURL(url=>!url.pathname.includes('login'));
 await page.goto('http://127.0.0.1:3000/d/agenova/agenova-work-diagnostics');await page.getByText('Work results',{exact:true}).waitFor();await delay(5000);
 // Grafana virtualizes panels; a full-page resize can unmount them during capture.
 await page.screenshot({path:resolve(directory,'grafana.png'),animations:'disabled'});writeFileSync(resolve(directory,'grafana-render.txt'),await page.locator('body').innerText());
 await page.goto('http://127.0.0.1:5177/?mode=connected#/history');await page.getByRole('heading',{name:'Work history',exact:true}).waitFor();await page.getByRole('link',{name:result.success,exact:true}).waitFor();await page.screenshot({path:resolve(directory,'portal-history.png'),animations:'disabled'});
 for(const [label,ref] of [['success',result.success],['runtime-failure',faults?.runtime],['provider-failure',faults?.provider]]){
  if(!ref)continue;await page.goto(`http://127.0.0.1:5177/?mode=connected#/work/${ref}`);await page.getByRole('link',{name:'Open trace in Grafana',exact:true}).waitFor();await page.screenshot({path:resolve(directory,`portal-${label}.png`),animations:'disabled'});
  const response=await page.request.get(`http://127.0.0.1:5177/api/requests/${ref}/evidence`);const view=await response.json();if(view.requestRef!==ref)throw new Error('Portal/API reference mismatch');
  if(label==='success'){
   const popupReady=page.waitForEvent('popup');await page.getByRole('link',{name:'Open trace in Grafana',exact:true}).click();const tracePage=await popupReady;
   await tracePage.waitForLoadState('domcontentloaded');
   try{await tracePage.getByText('agenova-control-plane',{exact:false}).first().waitFor({timeout:15000})}catch(error){await tracePage.screenshot({path:resolve(directory,'grafana-trace-debug.png'),animations:'disabled'});writeFileSync(resolve(directory,'trace-debug.txt'),await tracePage.locator('body').innerText());throw error}

   await delay(1500);await tracePage.screenshot({path:resolve(directory,'grafana-trace.png'),animations:'disabled'});
   writeFileSync(resolve(directory,'trace-navigation.json'),JSON.stringify({requestRef:ref,traceId:view.traceId,destination:tracePage.url(),renderedService:'agenova-control-plane'},null,2)+'\n');await tracePage.close();
  }

 }
 console.log('PASS: rendered Grafana dashboard, durable history and available Work diagnostics.');
}finally{await browser?.close();for(const child of children)child.kill()}

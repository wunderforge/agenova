// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Deletes only the five harness-owned telemetry components, never namespace/PVCs.
import {execFileSync} from 'node:child_process';
import {readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
const i=process.argv.indexOf('--context'),context=process.argv[i+1];if(i<0||!context)throw new Error('Explicit --context required');
const namespace='agenova-observability-180',directory=resolve('.tmp/observability180'),cli=resolve('.tmp/agenova-180'),state=resolve(directory,'state');
const kubectl=(...args)=>execFileSync('kubectl',['--context',context,'-n',namespace,...args],{encoding:'utf8',timeout:180000});
const run=(...args)=>execFileSync(cli,[...args,'--state-dir',state],{encoding:'utf8',timeout:600000,maxBuffer:8*1024*1024});
const page=JSON.parse(run('work','history','--json'));if(page.nextCursor||page.items.some(x=>['Pending','Bound','Running'].includes(x.status)))throw new Error('Verify all Work is terminal before cleanup');
const names=['agenova-otel','agenova-grafana','agenova-tempo','agenova-loki','agenova-prometheus'];
const before=JSON.parse(kubectl('get','pvc','-o','json')).items.map(item=>({name:item.metadata.name,uid:item.metadata.uid}));
for(const name of names){for(const [kind,resource] of [['deployment',name],['service',name],['configmap',name+'-config']]){const object=JSON.parse(kubectl('get',kind,resource,'-o','json'));if(object.metadata.labels?.['app.kubernetes.io/managed-by']!=='agenova')throw new Error('Unowned resource; refusing cleanup');}}
console.log('Deleting only the five owned telemetry components; retaining all data volumes…');
for(const kind of ['deployment','service'])kubectl('delete',kind,...names,'--wait=true','--timeout=90s');
kubectl('delete','configmap',...names.map(name=>name+'-config'));
const retained=JSON.parse(kubectl('get','pvc','-o','json')).items.map(item=>({name:item.metadata.name,uid:item.metadata.uid}));if(JSON.stringify(before)!==JSON.stringify(retained))throw new Error('Cleanup changed PVC identity');
const applied=JSON.parse(run('platform','apply','-f',resolve(directory,'platform.yaml'),'--yes','--json'));
if(!applied.ready)throw new Error('Reinstall did not become Ready');
const result=JSON.parse(readFileSync(resolve(directory,'result.json'),'utf8'));
const recovered=JSON.parse(run('work','show',result.success,'--json'));if(recovered.outcome?.status!=='Succeeded')throw new Error('Reinstall lost evidence');
writeFileSync(resolve(directory,'cleanup-result.json'),JSON.stringify({context,namespace,retainedPVCs:retained,reinstalled:true,requestRef:recovered.requestRef,outcome:recovered.outcome.status},null,2)+'\n');
console.log('PASS: telemetry cleanup retained PVC identities; Platform reinstalled and historical Work remains queryable.');

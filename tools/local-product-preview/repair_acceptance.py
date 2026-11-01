# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Opt-in real local inference acceptance; never a cloud deployment test."""
import argparse,urllib.request,urllib.error,urllib.parse,http.cookiejar,json,time,pathlib,sys,uuid
sys.stdout.reconfigure(encoding='utf-8')
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--execute',action='store_true',help='Explicitly submit bounded local model tasks')
parser.add_argument('--case',choices=['benchmark','weather','calendar','python','crm','denial','failure'],default='benchmark')
args=parser.parse_args()
if not args.execute: parser.error('--execute is required; this runs real local inference')
base='http://127.0.0.1:5226'
jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
client.open(base+'/login').read()
outdir=pathlib.Path('.tmp/repair-acceptance');outdir.mkdir(exist_ok=True)
def api(path,body=None,advice=None):
 req=urllib.request.Request(base+path,data=json.dumps(body).encode() if body is not None else None,headers={'Content-Type':'application/json',**({'X-Agenova-Advice-ID':advice} if advice else {})})
 try:
  with client.open(req,timeout=150) as response:return json.load(response)
 except urllib.error.HTTPError as e:return {'httpError':e.code,'detail':e.read().decode()}
def run(label,goal,tools=None,mode="automatic"):
 ref='work-'+str(uuid.uuid4());request={'apiVersion':'agenova.io/v1alpha1','kind':'ClaimRequest','metadata':{'name':ref},'spec':{'templateRef':'researcher-advisor-v4','projectRef':'research','task':{'input':{'objective':goal}},'runtime':{'maxModelCalls':6,'profileRef':'standard-isolated','timeout':'3m'}}}
 if tools is not None:request['spec']['requestedAccess']={'tools':tools,'resourceScopes':['crm:customer-retention'] if tools==['crm.read'] else ['web:public-pages'],'modelProfile':'approved-general-model'}
 begin=time.monotonic()
 if mode=='suggest':
  plan=api('/api/access-advice',{'request':request,'mode':'suggest'})
  if plan.get('status')!='ready':raise RuntimeError('Suggestion did not produce a ready request: '+str(plan))
  view=api('/api/requests',plan['request'],plan['id'])
 else: view=api('/api/access-submit',{'request':request,'mode':'automatic'}) if tools is None else api('/api/requests',request)
 submitted=time.monotonic()
 if 'httpError' in view or 'requestRef' not in view:print(json.dumps({'label':label,'submission':view}),flush=True);return
 while time.monotonic()-begin<180:
  view=api('/api/requests/'+view['requestRef']+'/evidence')
  if view.get('outcome'):break
  time.sleep(.35)
 content={}
 for f in view.get('facts',[]):
  if f.get('contentRef'):content[f['contentRef']]=api('/api/requests/'+view['requestRef']+'/content/'+urllib.parse.quote(f['contentRef']))
 payload={'view':view,'content':content};(outdir/(label+'.json')).write_text(json.dumps(payload,ensure_ascii=False,indent=2),encoding='utf-8')
 summary={'label':label,'ref':ref,'wallSeconds':round(time.monotonic()-begin,2),'submitSeconds':round(submitted-begin,2),'granted':view.get('state',{}).get('effectiveAuthority',{}).get('tools'),'outcome':view.get('outcome'),'models':sum(f['kind']=='ProviderAttempt' and f.get('operation')=='model.invoke' for f in view.get('facts',[])),'tools':[(f.get('target'),f.get('providerStatus'),f.get('reasonCode')) for f in view.get('facts',[]) if f['kind']=='ProviderOutcome' and f.get('operation')=='tool.invoke'],'checks':[(f.get('reasonCode'),f.get('reason')) for f in view.get('facts',[]) if f.get('operation')=='AgentFeedback'],'timing':[{'elapsed':c.get('elapsedMs'),'load':c.get('loadMs'),'prompt':c.get('promptMs'),'gen':c.get('generateMs'),'inputTokens':c.get('inputTokens'),'outputTokens':c.get('outputTokens'),'stop':c.get('finishReason')} for c in content.values() if c.get('kind')=='model-response']}
 print(json.dumps(summary,ensure_ascii=False),flush=True);return summary

# Compare answer data and platform evidence independently. Do not call a green
# execution state proof of goal completion or packet-level isolation.
CASES={
 'weather':('What is the weather in Sydney now?',None,'automatic'),
 'calendar':('When is Chinese New Year in 2027?',None,'automatic'),
 'python':('Explain what Python is used for, based on its official website.',['web.search','web.read'],'manual'),
 'crm':('From our internal customer data, find three at-risk customers and suggest next steps.',None,'suggest'),
 'denial':('What is the weather in Sydney now?',['crm.read'],'manual'),
 'failure':('Read only https://www.python.org/agenova-demo-page-not-found-20261101 and report whether that page was accessible.',['web.read'],'manual'),
}
selected=list(CASES)[:-1]*2 if args.case=='benchmark' else [args.case]
results=[]
for index,name in enumerate(selected):
 goal,tools,mode=CASES[name]
 r=run(f'{name}-{index+1}',goal,tools,mode)
 if not r:raise RuntimeError('No Work result')
 payload=json.loads((outdir/(r['label']+'.json')).read_text(encoding='utf-8'))
 view=payload['view'];outcome=view.get('outcome',{});facts=view.get('facts',[])
 if name=='denial':
  assert outcome.get('completion',{}).get('reason')=='tool-denied',r
  assert any(f['kind']=='ToolDecision' and f.get('result')=='Deny' for f in facts),r
  assert not any(f['kind']=='ProviderAttempt' and f.get('operation')=='tool.invoke' for f in facts),r
  import re
  startup=pathlib.Path('.tmp/researcher-v4-stdout.log').read_text(encoding='utf-8')
  match=re.search(r'receipts ([^;]+);',startup)
  assert match,'Owned MCP receipt file was not identified'
  receipt=pathlib.Path(match.group(1).strip())
  assert receipt.resolve().parent==outdir.parent.resolve(),'Unexpected receipt path'
  denied={f['invocationId'] for f in facts if f['kind']=='ToolDecision' and f.get('result')=='Deny' and f.get('invocationId')}
  assert denied,'No invocation correlation for the actual denial'
  backend=[json.loads(line) for line in receipt.read_text(encoding='utf-8').splitlines() if line.strip()]
  assert not any(e.get('event')=='tool' and e.get('correlation') in denied for e in backend),'Denied call reached MCP'
  r['independentDeniedBackendCalls']=0

 elif name=='failure':
  assert any(f.get('reasonCode')=='source-unavailable' and '404' in f.get('reason','') for f in facts),r
  assert outcome.get('status')=='Succeeded',r
 else:
  assert outcome.get('status')=='Succeeded' and outcome.get('completion',{}).get('status')=='completed',r
  assert r['models']<=3,r
  if name=='weather':
   import re
   reads=[json.loads(c['output']) for c in payload['content'].values() if c.get('kind')=='tool-response' and 'wttr.in' in c.get('output','')]
   assert reads,r
   temp=re.search(r'"temp_C"\s*:\s*"(-?\d+)"',reads[0]['content'])
   assert temp and temp.group(1) in outcome.get('text',''),r
  if name=='calendar':assert '2027' in outcome.get('text','') and ('February 6' in outcome.get('text','') or '6 February' in outcome.get('text','')),r
  if name=='python':assert any(c.get('kind')=='tool-response' and 'python.org' in c.get('output','') for c in payload['content'].values()),r
  if name=='crm':assert any(verb in outcome.get('text','').lower() for verb in ['contact','offer','follow-up','check-in','review','schedule','training','reach out','call','outreach','engage']),r
  if name=='crm':assert all(t[0]=='crm.read' for t in r['tools']) and r['tools'] and any(x in outcome.get('text','').lower() for x in ['synthetic','sample','demo data']),r
 results.append(r)
(outdir/'summary.json').write_text(json.dumps(results,ensure_ascii=False,indent=2),encoding='utf-8')
ordered=sorted(r['wallSeconds'] for r in results)
print(json.dumps({'accepted':len(results),'p50Seconds':ordered[(len(ordered)-1)//2],'p95Seconds':ordered[min(len(ordered)-1,__import__('math').ceil(len(ordered)*.95)-1)]}),flush=True)

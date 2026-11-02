#!/usr/bin/env python3
"""Supplementary #198 admission and cross-install cases; run after the primary campaign."""
from pathlib import Path
import json, subprocess, time
import argparse
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--output', required=True)
p.add_argument('--state-a', required=True)
p.add_argument('--state-b', required=True)
p.add_argument('--cli', default='.tmp/agenova')
args=p.parse_args()
out=Path(args.output)
cli=args.cli
a=args.state_a; b=args.state_b
def cli_run(args,label):
 r=subprocess.run([cli]+args,capture_output=True,text=True,timeout=180)
 (out/(label+'.json')).write_text(r.stdout)
 (out/(label+'.stderr.txt')).write_text(r.stderr)
 return r

def show(ref,state,label):
 r=cli_run(['work','show',ref,'--state-dir',state,'--json'],label)
 return json.loads(r.stdout or '{}')

def count():
 return sum('[GIN]' in l and '/v1/chat/completions' in l for l in (Path.home()/'.ollama/logs/server.log').read_text().splitlines())
base=json.loads((out/'profile.request.json').read_text())
rows=[]
for name in ['denied-submission','credential-canary']:
 req=json.loads(json.dumps(base));req['metadata']['name']='e15-198-'+name
 if name=='denied-submission': req['spec']['projectRef']='billing'
 else: req['spec']['task']['input']['apiKey']='E15_SYNTHETIC_CANARY_NOT_A_SECRET'
 path=out/(name+'.request.json');path.write_text(json.dumps(req,indent=2)+'\n')
 before=count();r=cli_run(['run','-f',str(path),'--state-dir',a,'--json'],name+'.run');after=count()
 rows.append({'scenario':name,'exit':r.returncode,'before':before,'after':after,'delta':after-before})
 show(req['metadata']['name'],a,name+'.show')
(out/'admission-counts.json').write_text(json.dumps(rows,indent=2)+'\n')
# B pauses inside its bound Running session. Observe its first provider
# outcome before measuring A, so the windows do not overlap provider traffic.
req=json.loads(json.dumps(base));req['metadata']['name']='e15-198-b-running'
req['spec']['task']['input']['objective']='e15-scenario=after-expiry wait=90s'
path=out/'b-running.request.json';path.write_text(json.dumps(req,indent=2)+'\n')
with (out/'b-running.run.json').open('w') as fo, (out/'b-running.run.stderr.txt').open('w') as fe:
 proc=subprocess.Popen([cli,'run','-f',str(path),'--state-dir',b,'--json'],stdout=fo,stderr=fe)
 try:
  deadline=time.monotonic()+60
  while True:
   view=show('e15-198-b-running',b,'b-running.before')
   facts=view.get('facts',[])
   phase=((view.get('state') or {}).get('claim') or {}).get('phase')
   if phase=='Running' and any(f.get('kind')=='ProviderOutcome' for f in facts): break
   if time.monotonic()>deadline: raise RuntimeError('B did not reach paused Running')
   time.sleep(1)
  req=json.loads(json.dumps(base));req['metadata']['name']='e15-198-cross-install'
  req['spec']['task']['input']['objective']='e15-scenario=cross-claim target='+view['state']['claim']['id']
  path=out/'cross-install.request.json';path.write_text(json.dumps(req,indent=2)+'\n')
  before=count();r=cli_run(['run','-f',str(path),'--state-dir',a,'--json'],'cross-install.run');after=count()
  av=show('e15-198-cross-install',a,'cross-install.show')
  bv=show('e15-198-b-running',b,'b-running.after')
  result={'before':before,'after':after,'delta':after-before,'aPhase':av['state']['claim']['phase'],'bPhaseBefore':view['state']['claim']['phase'],'bPhaseAfter':bv['state']['claim']['phase'],'bFactsUnchanged':view['facts']==bv['facts']}
  (out/'cross-install-counts.json').write_text(json.dumps(result,indent=2)+'\n');print(result,flush=True)
  proc.wait(timeout=120)
 finally:
  if proc.poll() is None: proc.terminate();proc.wait(timeout=10)
show('e15-198-b-running',b,'b-running.final')
print(rows,flush=True)

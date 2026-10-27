#!/usr/bin/env python3
"""Run bounded #198 scenarios on an already configured disposable test install.
Requires an explicit state directory and evidence directory. Does not install,
change image configuration, or remove cluster objects. No findings are asserted
from a zero count alone; inspect the evidence and qualification in summary.md.
"""
import argparse
import json
import subprocess
from pathlib import Path


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--state-dir', required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--cli', default='.tmp/agenova')
    p.add_argument('--ollama-log', default=str(Path.home()/'.ollama/logs/server.log'))
    args = p.parse_args()
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)

    def command(argv, name):
        r = subprocess.run(argv, capture_output=True, text=True, timeout=240)
        (out/(name+'.json')).write_text(r.stdout)
        (out/(name+'.stderr.txt')).write_text(r.stderr)
        return r

    def count():
        return sum('[GIN]' in line and '/v1/chat/completions' in line for line in Path(args.ollama_log).read_text().splitlines())

    base = {'apiVersion':'agenova.io/v1alpha1','kind':'ClaimRequest','metadata':{},'spec':{
        'templateRef':'engineer','projectRef':'payments','task':{'type':'investigation','input':{}},
        'requestedAccess':{'tools':['git.read'],'resourceScopes':['repo:acme/payments'],'modelProfile':'coding-standard'},
        'runtime':{'profileRef':'standard-isolated','timeout':'3m'}}}
    rows = []
    target = None
    scenarios = ['control', 'profile','tool','scope','malformed','unsupported','turn-limit','cross-claim','after-expiry','after-failure']
    for scene in scenarios:
        request = json.loads(json.dumps(base))
        ref = 'e15-198-'+scene
        request['metadata']['name'] = ref
        objective = 'e15-scenario='+scene
        if scene == 'control':
            request['spec']['requestedAccess']['tools'] = []
            request['spec']['requestedAccess']['resourceScopes'] = []
        if scene == 'cross-claim':
            if target is None:
                raise RuntimeError('no successful same-install target for cross-claim')
            objective += ' target='+target
        if scene == 'after-expiry':
            objective += ' wait=30s'
            request['spec']['runtime']['timeout'] = '15s'
        request['spec']['task']['input']['objective'] = objective
        path = out/(scene+'.request.json')
        path.write_text(json.dumps(request, indent=2)+'\n')
        before = count()
        r = command([args.cli,'run','-f',str(path),'--state-dir',args.state_dir,'--json'],scene+'.run')
        after = count()
        query = command([args.cli,'work','show',ref,'--state-dir',args.state_dir,'--json'],scene+'.show')
        view = json.loads(query.stdout or '{}')
        claim = (view.get('state') or {}).get('claim') or {}
        facts = view.get('facts') or []
        row = {'scenario':scene,'exit':r.returncode,'before':before,'after':after,'delta':after-before,
               'phase':claim.get('phase'),'outcome':view.get('outcome'),
               'modelAttempts':sum(f.get('kind')=='ProviderAttempt' and f.get('operation')=='model.invoke' for f in facts),
               'denials':[f for f in facts if f.get('result')=='Deny'],
               'runReasons':[f.get('reasonCode') for f in facts if f.get('kind')=='RunOutcome']}
        rows.append(row)
        (out/'counts.json').write_text(json.dumps(rows,indent=2)+'\n')
        print(json.dumps({k:v for k,v in row.items() if k not in ('outcome','denials')}),flush=True)
        if scene == 'control':
            if claim.get('phase') != 'Succeeded' or after-before != row['modelAttempts'] or after <= before:
                raise RuntimeError('positive control failed; stop campaign')
            target=claim['id']
        if scene == 'cross-claim':
            command([args.cli,'work','show','e15-198-control','--state-dir',args.state_dir,'--json'],'cross-claim.target-after')


if __name__ == '__main__':
    main()

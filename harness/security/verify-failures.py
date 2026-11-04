#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Check retained real #199 campaign evidence. Does not generate mock evidence."""
import argparse
import json
from pathlib import Path


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['campaign','cold','terminal']:p.add_argument('--'+name,required=True)
    a=p.parse_args();root=Path(a.campaign);cold=Path(a.cold);terminal=Path(a.terminal)
    def read(path):return json.loads(path.read_text())
    expected={'healthy':'Succeeded','slow':'Expired','queue-expiry':'Expired','queue-blocker':'Succeeded','queue-resume':'Succeeded','worker-delete':'Failed','worker-recovery':'Succeeded','controller-short':'Succeeded','controller-queue':'Succeeded','controller-running':'Succeeded','restart-recovery':'Succeeded'}
    results=read(root/'results.json')
    for case,status in expected.items():
        v=read(root/(case+'.final.json'))['body'];submitted=read(root/(case+'.submission.json'))['body']
        assert v['outcome']['status']==status,(case,v['outcome'])
        assert v['state']['claim']['phase']==status
        assert v['state']['effectiveAuthority']==submitted['state']['effectiveAuthority'],case+' authority changed'
        assert results[case]['noUnsupportedSuccess'],case+' unsupported success'
        attempts=[f for f in v['facts'] if f['kind']=='ProviderAttempt' and f.get('operation')=='model.invoke']
        outcomes=[f for f in v['facts'] if f['kind']=='ProviderOutcome' and f.get('operation')=='model.invoke']
        assert len(attempts)==len(outcomes)
        assert {f['invocationId'] for f in attempts}=={f['invocationId'] for f in outcomes}
        if case=='queue-expiry':
            assert not v['state']['claim'].get('backendIdentity') and not attempts
        else:
            assert v['state']['claim']['backendIdentity']['backend']=='agent-sandbox'
            assert any(f.get('operation')=='TerminateSucceeded' for f in v['facts'])
            assert any(f.get('operation')=='CleanupSucceeded' for f in v['facts'])
        if status!='Succeeded':assert not v['outcome'].get('text')
    # Windows with two queued Works share the same independent provider counter.
    for cases in [('healthy',),('slow',),('queue-blocker','queue-resume'),('worker-delete',),('worker-recovery',),('controller-short','controller-queue'),('controller-running',),('restart-recovery',)]:
        first=results[cases[0]]
        completed=first['providerAfter']['completed']-first['providerBefore']['completed']
        journal=sum(sum(x=='Succeeded' for x in results[c]['providerOutcomes']) for c in cases)
        assert completed==journal,(cases,completed,journal)
    for case in ['restart-active','restart-queued']:
        response=read(root/(case+'.after.json'));assert response['status']==404
        assert results[case]['outcome']=='unknown/lost'
    for case in ['cold-allocation','cold-queue']:
        v=read(cold/(case+'.final.json'))
        assert v['state']['claim']['phase']=='Expired' and v['outcome']['status']=='Expired'
        assert not v['state']['claim'].get('backendIdentity')
        assert not any(f['kind']=='ProviderAttempt' for f in v['facts'])
        restored=read(cold/(case+'.restored.json'));assert restored['status']==200 and restored['body']==v
    for mode,status in [('succeeded','Succeeded'),('failed','Failed'),('expired','Expired')]:
        v=read(terminal/(mode+'.json'));assert v['evidence']['outcome']['status']==status
        assert v['factsUnchanged'] and v['providerBeforeReplay']==v['providerAfterReplay']
        assert len(v['replays'])==2 and all(not x['allowed'] and x['error'] for x in v['replays'])
    print('PASS: installed fault outcomes, unchanged authority, exact provider windows, queue expiry, explicit restart loss, cold expiry and supplementary real-backend terminal replay.')


if __name__=='__main__':main()

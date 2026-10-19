#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Opt-in real-provider RI-3 campaign. Requires an owned, already installed
kind namespace and a dedicated Ollama container. Never stops the host service.
Run from repository root; raw outputs and assertions are retained on failure.
"""
import argparse
import json
import subprocess
import time
from pathlib import Path


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--container-id', required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--state-dir', required=True)
    p.add_argument('--cli', default='.tmp/agenova')
    a = p.parse_args()
    out = Path(a.output)
    out.mkdir(parents=True, exist_ok=True)
    k = ['kubectl', '--context', 'kind-agenova-k8s-lab', '-n', 'agenova-e15-199-provider']

    def cmd(argv, label, check=True):
        r = subprocess.run(argv, text=True, capture_output=True, timeout=420)
        (out/(label+'.stdout')).write_text(r.stdout)
        (out/(label+'.stderr')).write_text(r.stderr)
        (out/(label+'.command.json')).write_text(json.dumps({'argv': argv, 'exit': r.returncode}, indent=2)+'\n')
        if check and r.returncode:
            raise RuntimeError(label+' failed; inspect retained stderr')
        return r

    def save(name, value):
        (out/(name+'.json')).write_text(json.dumps(value, indent=2)+'\n')

    identity = json.loads(cmd(['docker', 'inspect', a.container_id], 'provider.before').stdout)[0]
    assert identity['Id'] == a.container_id
    assert identity['Config']['Labels'].get('agenova.ticket') == '199'
    assert identity['Name'] == '/agenova-e15-199-provider'
    assert identity['State']['Running']
    cli = [a.cli]
    def call(argv, label, check=True):
        return cmd(cli+argv+['--state-dir', a.state_dir, '--json'], label, check)

    def logs(label):
        r = cmd(['docker', 'logs', a.container_id], label)
        lines = (r.stdout+r.stderr).splitlines()
        chats = [line for line in lines if '[GIN]' in line and '/v1/chat/completions' in line]
        completed = [line for line in chats if '| 200 |' in line]
        return {'requests': len(chats), 'completed': len(completed)}

    request = {'apiVersion':'agenova.io/v1alpha1','kind':'ClaimRequest','metadata':{},'spec':{
        'templateRef':'engineer','projectRef':'payments',
        'task':{'type':'investigation','input':{'objective':'Explain in one sentence why synthetic payment retries need a deadline. Do not use tools.'}},
        'requestedAccess':{'tools':[],'resourceScopes':[],'modelProfile':'coding-standard'},
        'runtime':{'profileRef':'standard-isolated','timeout':'5m'}}}
    run_id = str(time.time_ns())
    results = {}
    stopped = False
    try:
        for scenario in ['healthy','outage','recovery']:
            if scenario == 'outage':
                cmd(['docker','stop', '--time','10',a.container_id], 'provider.stop')
                stopped = True
            if scenario == 'recovery':
                cmd(['docker','start',a.container_id], 'provider.start')
                stopped = False
                # Container readiness is independently checked before Work.
                for attempt in range(30):
                    r=cmd(['docker','exec',a.container_id,'ollama','list'], 'provider.ready-'+str(attempt),False)
                    if r.returncode == 0:break
                    time.sleep(1)
                else:raise RuntimeError('provider did not recover')
            state=json.loads(cmd(['docker','inspect',a.container_id],scenario+'.provider-state').stdout)[0]['State']
            assert state['Running'] == (scenario != 'outage')
            probe=cmd(['docker','exec','agenova-k8s-lab-control-plane','curl','-fsS','--max-time','5','http://host.docker.internal:11439/api/tags'],scenario+'.provider-probe',False)
            assert (probe.returncode == 0) == (scenario != 'outage')
            before=logs(scenario+'.provider-log-before')
            request['metadata']['name']='e15-199-'+scenario+'-'+run_id
            path=out/(scenario+'.request.json');path.write_text(json.dumps(request,indent=2)+'\n')
            r=call(['run','-f',str(path)],scenario+'.run',False)
            view=json.loads(call(['work','show',request['metadata']['name']],scenario+'.show').stdout)
            save(scenario+'.evidence',view)
            after=logs(scenario+'.provider-log-after')
            facts=view['facts'];outcome=view['outcome']['status']
            models=[f for f in facts if f['kind']=='ProviderOutcome' and f.get('operation')=='model.invoke']
            attempts=[f for f in facts if f['kind']=='ProviderAttempt' and f.get('operation')=='model.invoke']
            assert len(attempts) == len(models) > 0
            assert {f['invocationId'] for f in attempts} == {f['invocationId'] for f in models}
            assert view['state']['claim']['phase'] == outcome
            assert view['state']['decision']['result']=='Allow'
            assert view['state']['claim']['backendIdentity']['backend']=='agent-sandbox'
            assert any(f.get('operation')=='TerminateSucceeded' for f in facts)
            assert any(f.get('operation')=='CleanupSucceeded' for f in facts)
            if scenario=='outage':
                assert outcome=='Failed', outcome
                assert models and all(f['providerStatus']=='Failed' for f in models)
                assert not view['outcome'].get('text')
                assert after==before
            else:
                assert outcome=='Succeeded', outcome
                assert models and all(f['providerStatus']=='Succeeded' for f in models)
                assert after['completed']-before['completed']==len(models)>0
            cmd(k+['get','pods,sandboxclaims,sandboxes','-o','json'],scenario+'.resources')
            results[scenario]={'outcome':outcome,'cliExit':r.returncode,'providerBefore':before,'providerAfter':after,'attempts':len(attempts),'modelOutcomes':len(models)}
            save('results',results)
    finally:
        if stopped:cmd(['docker','start',a.container_id], 'provider.emergency-restart',False)


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""#199 sustained cold-allocation controller outage on the dedicated kind only.
Precondition: component-failures campaign finished, dedicated provider running.
No product hooks; real controller scaled to zero and restored in finally.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--owner',required=True);p.add_argument('--kubeconfig',required=True);p.add_argument('--output',required=True)
    a=p.parse_args();out=Path(a.output);out.mkdir(parents=True,exist_ok=False)
    owner=json.loads(Path(a.owner).read_text());k=['kubectl','--kubeconfig',a.kubeconfig,'--context','kind-agenova-e15-199-faults'];os.environ['KUBECONFIG']=str(Path(a.kubeconfig).resolve())
    ns='agenova-e15-199-cold';state='.tmp/faults/state-cold'
    def save(label,v):(out/(label+'.json')).write_text(json.dumps(v,indent=2)+'\n')
    def cmd(argv,label,check=True):
        r=subprocess.run(argv,text=True,capture_output=True,timeout=180)
        (out/(label+'.stdout')).write_text(r.stdout);(out/(label+'.stderr')).write_text(r.stderr);save(label+'.command',{'argv':argv,'exit':r.returncode,'time':time.time()})
        if check and r.returncode:raise RuntimeError(label+' failed')
        return r
    identity=json.loads(cmd(k+['get','namespace','kube-system','-o','json'],'cluster-identity').stdout)
    assert identity['metadata']['uid']==owner['kubeSystemUID']
    cmd(k+['create','namespace',ns,'-o','json'],'namespace-created')
    text=Path('docs/evidence/199/full/controller.platform.yaml').read_text().replace('e15-199-controller','e15-199-cold');f=out/'platform.yaml';f.write_text(text)
    cmd(['.tmp/agenova','platform','apply','-f',str(f),'--state-dir',state,'--yes','--json'],'installed')
    for name,path in [('policy','policy'),('agent-template','engineer')]:cmd(['.tmp/agenova',name,'apply','-f','deploy/reference/demo/'+path+'.yaml','--state-dir',state,'--json'],name)
    handle=(out/'forward.log').open('w');proc=subprocess.Popen(k+['-n',ns,'port-forward','deployment/agenova-control-plane','18101:8081','--address','127.0.0.1'],stdout=handle,stderr=handle,text=True)
    stopped=False
    def api(path,body=None):
        req=urllib.request.Request('http://127.0.0.1:18101'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
        try:
            with urllib.request.urlopen(req,timeout=8) as r:return r.status,json.load(r)
        except urllib.error.HTTPError as e:return e.code,json.load(e)
        except (urllib.error.URLError,TimeoutError,ConnectionError):return 0,{}
    def snapshot(label):
        cmd(k+['-n',ns,'get','pods,sandboxclaims,sandboxes,sandboxwarmpools','-o','json'],label+'.resources')
        cmd(['docker','logs',owner['providerID']],label+'.provider-log')
    try:
        for _ in range(50):
            if api('/api/setup')[0]==200:break
            time.sleep(.2)
        else:raise RuntimeError('service unavailable')
        snapshot('before')
        cmd(k+['-n','agent-sandbox-system','scale','deployment/agent-sandbox-controller','--replicas=0'],'controller-stop');stopped=True
        cmd(k+['-n','agent-sandbox-system','wait','--for=delete','pod','-l','app=agent-sandbox-controller','--timeout=60s'],'controller-absent')
        refs={}
        for label,timeout in [('cold-allocation','15s'),('cold-queue','8s')]:
            req={'apiVersion':'agenova.io/v1alpha1','kind':'ClaimRequest','metadata':{'name':'e15-199-'+label+'-'+str(time.time_ns())},'spec':{'templateRef':'engineer','projectRef':'payments','task':{'type':'investigation','input':{'objective':'Explain synthetic retry deadlines without tools.'}},'requestedAccess':{'modelProfile':'coding-standard'},'runtime':{'profileRef':'standard-isolated','timeout':timeout}}}
            save(label+'.request',req);status,v=api('/api/requests',req);save(label+'.submission',{'status':status,'body':v});assert status==202;refs[label]=req['metadata']['name']
        end=time.monotonic()+110;completed={}
        while time.monotonic()<end and len(completed)<2:
            for label,ref in refs.items():
                status,v=api('/api/requests/'+ref+'/evidence')
                if status==200 and v.get('outcome') is not None and label not in completed:
                    completed[label]=v;save(label+'.final',v)
                    r=cmd(['.tmp/agenova','work','show',ref,'--state-dir',state,'--json'],label+'.cli');assert json.loads(r.stdout)==v
            time.sleep(.5)
        save('results',completed);snapshot('during-after-deadline')
        cmd(k+['-n','agent-sandbox-system','scale','deployment/agent-sandbox-controller','--replicas=1'],'controller-restore');stopped=False
        cmd(k+['-n','agent-sandbox-system','rollout','status','deployment/agent-sandbox-controller','--timeout=90s'],'controller-restored')
        time.sleep(8);snapshot('restored')
        for label,ref in refs.items():save(label+'.restored',{'status':api('/api/requests/'+ref+'/evidence')[0],'body':api('/api/requests/'+ref+'/evidence')[1]})
        save('observations',{'allTerminal':len(completed)==2,'noSucceeded':all(v['outcome']['status']!='Succeeded' for v in completed.values()),'workSpecDeadlines':['15s','8s'],'allocationAdapterWait':'60s synchronous boundary; not immediate deadline interruption'})
        assert len(completed)==2
    finally:
        if stopped:
            cmd(k+['-n','agent-sandbox-system','scale','deployment/agent-sandbox-controller','--replicas=1'],'finally-controller-restore',False)
        proc.terminate();proc.wait(timeout=10);handle.close()


if __name__=='__main__':main()

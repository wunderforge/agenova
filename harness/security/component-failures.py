#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Opt-in #199 black-box matrix on the dedicated disposable kind cluster.
Requires preinstalled full/controller namespaces and a ticket-owned real Ollama
container. Raw evidence stays local until disclosure review. Product code is
not changed. Every kubectl call has an explicit kubeconfig and context.
"""
import argparse
import datetime
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--output', required=True)
    p.add_argument('--owner', required=True)
    p.add_argument('--kubeconfig', required=True)
    a = p.parse_args()
    out = Path(a.output)
    out.mkdir(parents=True, exist_ok=False)
    owner = json.loads(Path(a.owner).read_text())
    cid = owner['providerID']
    k = ['kubectl', '--kubeconfig', a.kubeconfig, '--context', 'kind-agenova-e15-199-faults']
    os.environ['KUBECONFIG'] = str(Path(a.kubeconfig).resolve())
    forwards = {}
    handles = []
    results = {}
    ports = {'full': 18099, 'controller': 18100}
    paused = False
    controller_stopped = False
    seq = 0

    def save(label, value):
        (out/(label+'.json')).write_text(json.dumps(value, indent=2)+'\n')

    def cmd(argv, label, body=None, check=True):
        nonlocal seq
        seq += 1
        started = datetime.datetime.now(datetime.timezone.utc).isoformat()
        r = subprocess.run(argv, input=body, text=True, capture_output=True, timeout=180)
        (out/(label+'.stdout')).write_text(r.stdout)
        (out/(label+'.stderr')).write_text(r.stderr)
        save(label+'.command', {'argv':argv, 'exit':r.returncode, 'started':started, 'sequence':seq})
        if check and r.returncode:
            raise RuntimeError(label+' failed; inspect retained stderr')
        return r

    def api(which, path, body=None):
        req=urllib.request.Request('http://127.0.0.1:'+str(ports[which])+path,
            data=None if body is None else json.dumps(body).encode(), headers={'Content-Type':'application/json'})
        try:
            with urllib.request.urlopen(req, timeout=8) as r:return r.status, json.load(r)
        except urllib.error.HTTPError as e:return e.code, json.load(e)
        except (urllib.error.URLError, TimeoutError, ConnectionError) as e:return 0, {'transportError':type(e).__name__}

    def forward(which):
        if which in forwards:
            forwards[which].terminate()
            forwards[which].wait(timeout=10)
        f=(out/(which+'.forward-'+str(time.time_ns())+'.log')).open('w');handles.append(f)
        proc=subprocess.Popen(k+['-n','agenova-e15-199-'+which,'port-forward','deployment/agenova-control-plane',str(ports[which])+':8081','--address','127.0.0.1'],stdout=f,stderr=f,text=True)
        forwards[which]=proc
        for _ in range(50):
            if proc.poll() is not None:raise RuntimeError('port forward failed')
            if api(which,'/api/setup')[0]==200:return
            time.sleep(.2)
        raise RuntimeError('installed service unavailable')

    def snapshots(label, which='full'):
        cmd(k+['-n','agenova-e15-199-'+which,'get','pods,sandboxclaims,sandboxes,sandboxwarmpools','-o','json'],label+'.resources')
        cmd(k+['-n','agenova-e15-199-'+which,'logs','deployment/agenova-control-plane'],label+'.control-log',check=False)
        r=cmd(['docker','logs',cid],label+'.provider-log')
        rows=[x for x in (r.stdout+r.stderr).splitlines() if '[GIN]' in x and '/v1/chat/completions' in x]
        counts={'responses':len(rows),'completed':sum('| 200 |' in x for x in rows)}
        save(label+'.counts',counts)
        return counts

    def submit(which, label, timeout='90s'):
        ref='e15-199-'+label+'-'+str(time.time_ns())
        req={'apiVersion':'agenova.io/v1alpha1','kind':'ClaimRequest','metadata':{'name':ref},'spec':{
            'templateRef':'engineer','projectRef':'payments','task':{'type':'investigation','input':{
            'objective':'Explain in one sentence why synthetic payment retries need a deadline. Do not use tools.'}},
            'requestedAccess':{'tools':[],'resourceScopes':[],'modelProfile':'coding-standard'},
            'runtime':{'profileRef':'standard-isolated','timeout':timeout}}}
        save(label+'.request',req)
        status,v=api(which,'/api/requests',req);save(label+'.submission',{'status':status,'body':v})
        assert status==202 and v['state']['decision']['result']=='Allow'
        return ref

    def query(which, ref):return api(which,'/api/requests/'+ref+'/evidence')

    def wait(which, ref, label, predicate, limit=110):
        end=time.monotonic()+limit;last=None
        while time.monotonic()<end:
            status,v=query(which,ref)
            if status==200 and predicate(v):
                save(label,{'status':status,'body':v});return v
            last={'status':status,'body':v};time.sleep(.4)
        save(label+'.timeout',last)
        raise RuntimeError(label+' not observed before bounded harness deadline')

    def terminal(which, ref, label, limit=110):
        v=wait(which,ref,label,lambda v: v.get('outcome') is not None,limit)
        # Compare connected CLI JSON to the exact installed API record.
        r=cmd(['.tmp/agenova','work','show',ref,'--state-dir','.tmp/faults/state-'+which,'--json'],label+'.cli')
        assert json.loads(r.stdout)==v
        return v

    def attempt(which,ref,label):
        return wait(which,ref,label,lambda v: v['state']['claim']['phase']=='Running' and any(f['kind']=='ProviderAttempt' for f in v.get('facts',[])))

    def summarize(label,v,before,after):
        facts=v.get('facts',[]);outcome=v.get('outcome',{})
        attempts=[f for f in facts if f['kind']=='ProviderAttempt' and f.get('operation')=='model.invoke']
        outcomes=[f for f in facts if f['kind']=='ProviderOutcome' and f.get('operation')=='model.invoke']
        results[label]={'requestRef':v['requestRef'],'claimPhase':v['state']['claim']['phase'],
            'outcome':outcome.get('status'),'answerPresent':bool(outcome.get('text')),
            'backendIdentity':v['state']['claim'].get('backendIdentity'),
            'providerBefore':before,'providerAfter':after,'attempts':len(attempts),
            'providerOutcomes':[f.get('providerStatus') for f in outcomes],
            'cleanup':[f.get('operation') for f in facts if f.get('operation','').startswith(('Cleanup','Terminate'))],
            'noUnsupportedSuccess':outcome.get('status')!='Succeeded' or after['completed']-before['completed']>=sum(f.get('providerStatus')=='Succeeded' for f in outcomes)>0}
        save('results',results)

    def pause():
        nonlocal paused
        cmd(['docker','pause',cid],'pause-'+str(time.time_ns()));paused=True

    def unpause():
        nonlocal paused
        cmd(['docker','unpause',cid],'unpause-'+str(time.time_ns()));paused=False

    def scale_controller(n):
        nonlocal controller_stopped
        cmd(k+['-n','agent-sandbox-system','scale','deployment/agent-sandbox-controller','--replicas='+str(n)],'controller-scale-'+str(time.time_ns()))
        controller_stopped=n==0
        if n==0:
            cmd(k+['-n','agent-sandbox-system','wait','--for=delete','pod','-l','app=agent-sandbox-controller','--timeout=60s'],'controller-stopped-'+str(time.time_ns()))
        else:
            cmd(k+['-n','agent-sandbox-system','rollout','status','deployment/agent-sandbox-controller','--timeout=90s'],'controller-restored-'+str(time.time_ns()))

    def delete_uid(namespace,name,uid,label):
        options={'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':uid},'gracePeriodSeconds':0}
        cmd(k+['delete','--raw','/api/v1/namespaces/'+namespace+'/pods/'+name,'-f','-'],label,json.dumps(options))

    try:
        c=json.loads(cmd(['docker','inspect',cid],'provider-identity').stdout)[0]
        assert c['Id']==cid and c['Config']['Labels']['agenova.ticket']=='199'
        assert c['Name']=='/agenova-e15-199-full-provider' and c['State']['Running']
        uid=json.loads(cmd(k+['get','namespace','kube-system','-o','json'],'cluster-identity').stdout)['metadata']['uid']
        assert uid==owner['kubeSystemUID']
        for which in ports:forward(which)
        # Healthy real model control on the new isolated substrate.
        before=snapshots('healthy.before');ref=submit('full','healthy')
        v=terminal('full',ref,'healthy.final');after=snapshots('healthy.after');summarize('healthy',v,before,after)
        assert v['outcome']['status']=='Succeeded' and results['healthy']['noUnsupportedSuccess']
        # Actual frozen provider process: timeout, plus queue expiry without allocation.
        pause();before=snapshots('slow.before');ref=submit('full','slow','35s');attempt('full',ref,'slow.running')
        queued=submit('full','queue-expiry','8s')
        q=terminal('full',queued,'queue-expiry.final',30);qcounts=snapshots('queue-expiry.after');summarize('queue-expiry',q,before,qcounts)
        v=terminal('full',ref,'slow.final',65);after=snapshots('slow.after');summarize('slow',v,before,after)
        unpause();time.sleep(3);snapshots('slow.restored')
        # A queued request survives a short provider freeze and runs once restored.
        pause();before=snapshots('queue-resume.before');ref=submit('full','queue-blocker');attempt('full',ref,'queue-blocker.running')
        queued=submit('full','queue-resume');time.sleep(2)
        save('queue-resume.pending',{'status':query('full',queued)[0],'body':query('full',queued)[1]})
        unpause();v=terminal('full',ref,'queue-blocker.final');q=terminal('full',queued,'queue-resume.final')
        after=snapshots('queue-resume.after');summarize('queue-blocker',v,before,after);summarize('queue-resume',q,before,after)
        # Worker disappears during an in-flight real model request.
        pause();before=snapshots('worker-delete.before');ref=submit('full','worker-delete','45s');v=attempt('full',ref,'worker-delete.running')
        worker=v['state']['claim']['backendIdentity']['workerId']
        pod=json.loads(cmd(k+['-n','agenova-e15-199-full','get','pod',worker,'-o','json'],'worker-delete.pod').stdout)
        delete_uid('agenova-e15-199-full',worker,pod['metadata']['uid'],'worker-delete.inject')
        unpause();v=terminal('full',ref,'worker-delete.final',90);after=snapshots('worker-delete.after');summarize('worker-delete',v,before,after)
        ref=submit('full','worker-recovery');v=terminal('full',ref,'worker-recovery.final');summarize('worker-recovery',v,after,snapshots('worker-recovery.after'))
        # Controller absent: new allocation blocked, queued Work waits, restored controller resumes both.
        scale_controller(0);before=snapshots('controller-short.before','controller')
        ref=submit('controller','controller-short');queued=submit('controller','controller-queue','100s');time.sleep(5)
        save('controller-short.pending',{'status':query('controller',ref)[0],'body':query('controller',ref)[1]})
        save('controller-queue.pending',{'status':query('controller',queued)[0],'body':query('controller',queued)[1]})
        snapshots('controller-short.during','controller');scale_controller(1)
        v=terminal('controller',ref,'controller-short.final');q=terminal('controller',queued,'controller-queue.final')
        after=snapshots('controller-short.after','controller');summarize('controller-short',v,before,after);summarize('controller-queue',q,before,after)
        # Cold template identity to avoid consuming an existing warm worker while controller is stopped.
        # A separate empty installation is required for the sustained cold-allocation outage;
        # this campaign records a Running workload with the controller absent instead.
        pause();before=snapshots('controller-running.before');ref=submit('full','controller-running');attempt('full',ref,'controller-running.running')
        scale_controller(0);unpause();v=terminal('full',ref,'controller-running.final',100)
        after=snapshots('controller-running.after');summarize('controller-running',v,before,after)
        scale_controller(1);time.sleep(3);snapshots('controller-running.restored')
        # Control-plane Pod restart interrupts active and queued Work; query loss explicitly.
        pause();before=snapshots('restart.before');ref=submit('full','restart-active');v=attempt('full',ref,'restart-active.running')
        queued=submit('full','restart-queued');save('restart-queued.before',{'status':query('full',queued)[0],'body':query('full',queued)[1]})
        pods=json.loads(cmd(k+['-n','agenova-e15-199-full','get','pods','-l','app.kubernetes.io/name=agenova-control-plane','-o','json'],'restart.control-pod').stdout)['items'];assert len(pods)==1
        pod=pods[0];delete_uid('agenova-e15-199-full',pod['metadata']['name'],pod['metadata']['uid'],'restart.inject')
        cmd(k+['-n','agenova-e15-199-full','rollout','status','deployment/agenova-control-plane','--timeout=90s'],'restart.ready')
        forward('full');unpause();time.sleep(5)
        for label,old in [('restart-active',ref),('restart-queued',queued)]:
            status,body= query('full',old);save(label+'.after',{'status':status,'body':body})
            results[label]={'requestRef':old,'postRestartHTTP':status,'outcome':'unknown/lost' if status==404 else body.get('outcome'), 'notInterpretedAsSuccess':status!=200 or body.get('outcome',{}).get('status')!='Succeeded'}
            cmd(['.tmp/agenova','work','show',old,'--state-dir','.tmp/faults/state-full','--json'],label+'.after.cli',check=False)
        after=snapshots('restart.after');save('results',results)
        ref=submit('full','restart-recovery');v=terminal('full',ref,'restart-recovery.final');summarize('restart-recovery',v,after,snapshots('restart-recovery.after'))
        save('completed',{'matrixExecutionComplete':True,'postTerminalTrustedReplay':'not exposed by installed API; unknown','sideEffectReplay':'no real side-effect adapter in this supported slice; unknown'})
    finally:
        if paused:cmd(['docker','unpause',cid],'finally-unpause',check=False)
        if controller_stopped:scale_controller(1)
        for proc in forwards.values():
            proc.terminate()
            try:proc.wait(timeout=10)
            except subprocess.TimeoutExpired:proc.kill();proc.wait()
        for f in handles:f.close()


if __name__=='__main__':
    main()

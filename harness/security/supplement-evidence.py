#!/usr/bin/env python3
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
"""Opt-in #198 evidence: continuous resource watches, bounded canary scan,
real Work capture, and rejection of unsupported Policy constraints.
Requires an already installed disposable namespace and private loopback tunnel.
Does not install or delete the namespace. No Secret values are collected.
"""
import argparse
import json
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for flag in ('context', 'namespace', 'state-dir', 'output', 'api-url'):
        parser.add_argument('--'+flag, required=True)
    parser.add_argument('--cli', default='.tmp/agenova')
    args = parser.parse_args()
    if args.context != 'kind-agenova-k8s-lab' or not args.namespace.startswith('agenova-e15-198-'):
        parser.error('Only the explicitly selected E15 disposable kind namespace is supported')
    if not args.api_url.startswith('http://127.0.0.1:'):
        parser.error('A literal loopback API endpoint is required')
    out = Path(args.output)
    out.mkdir(parents=True, exist_ok=True)
    marker = 'E15_198_SUPPLEMENT_SYNTHETIC_CANARY_NOT_A_SECRET'
    k = ['kubectl', '--context', args.context, '-n', args.namespace]

    def save(name, value):
        (out/name).write_text(json.dumps(value, indent=2)+'\n')

    def command(argv, label=None, input=None, check=True):
        r = subprocess.run(argv, input=input, text=True, capture_output=True, timeout=240)
        if label:
            (out/(label+'.stdout')).write_text(r.stdout)
            (out/(label+'.stderr')).write_text(r.stderr)
        if check and r.returncode:
            raise RuntimeError(f'{argv[:2]} failed ({r.returncode}): {r.stderr[:200]}')
        return r

    def get(resource):
        prefix='/apis/apps/v1' if resource=='deployments' else '/apis/agents.x-k8s.io/v1alpha1' if resource=='sandboxes' else '/apis/extensions.agents.x-k8s.io/v1alpha1' if resource in ('sandboxclaims','sandboxtemplates','sandboxwarmpools') else '/api/v1'
        uri=prefix+'/namespaces/'+args.namespace+'/'+resource
        return json.loads(command(['kubectl','--context',args.context,'get','--raw',uri]).stdout)

    def api(path, body=None):
        req = urllib.request.Request(args.api_url+path, data=body, headers={'Content-Type':'application/json'})
        try:
            response = urllib.request.urlopen(req, timeout=15)
        except urllib.error.HTTPError as e:
            response = e
        return response.status, json.loads(response.read())

    log = Path.home()/'.ollama/logs/server.log'
    def count():
        return sum('[GIN]' in line and '/v1/chat/completions' in line for line in log.read_text().splitlines())

    def cli(argv, label, check=True):
        return command([args.cli]+argv+['--state-dir', args.state_dir, '--json'],label,check=check)

    def capture(ref, label):
        v = json.loads(cli(['work','show',ref],label+'.cli').stdout)
        status, av = api('/api/requests/'+ref+'/evidence')
        if status != 200 or av != v:
            raise RuntimeError('CLI/API evidence mismatch')
        save(label+'.api.json', av)
        return v

    watch = []
    handles = []
    def events(resource):
        raw = (out/(resource+'.watch.jsonstream')).read_text()
        decoder = json.JSONDecoder()
        parsed = []
        while raw.strip():
            value, end = decoder.raw_decode(raw.lstrip())
            parsed.append(value)
            raw = raw.lstrip()[end:]
        return parsed

    def scan(label):
        resources = ['deployments','configmaps','pods','sandboxclaims','sandboxes','sandboxtemplates','sandboxwarmpools','serviceaccounts']
        report = {'markerKind':'synthetic canary','resources':{},'logs':{},'environment':{},'excluded':['Secret values','host filesystem','other namespaces','Kubernetes audit logs']}
        for resource in resources:
            items = get(resource)['items']
            report['resources'][resource] = {'objects':len(items),'matchingObjectNames':[i['metadata']['name'] for i in items if marker in json.dumps(i)]}
        for pod in get('pods')['items']:
            name=pod['metadata']['name']
            for container in pod['spec']['containers']:
                cname=container['name']
                r=command(k+['logs',name,'-c',cname],check=False)
                report['logs'][name+'/'+cname]={'readable':r.returncode==0,'canaryPresent':marker in r.stdout}
                if cname=='agent':
                    r=command(k+['exec',name,'-c',cname,'--','printenv'],check=False)
                    report['environment'][name+'/'+cname]={'readable':r.returncode==0,'canaryPresent':marker in r.stdout}
        save(label+'.json',report)
        return report

    base=json.loads(Path('docs/evidence/198/runtime/control.request.json').read_text())
    start=time.time()
    run_id=str(time.time_ns())
    try:
        for resource in ['pods','sandboxclaims']:
            initial=get(resource)
            if not initial['metadata'].get('resourceVersion'):raise RuntimeError('API list must carry resourceVersion')
            save(resource+'.initial.json',initial)
            fo=(out/(resource+'.watch.jsonstream')).open('w')
            fe=(out/(resource+'.watch.stderr')).open('w')
            handles += [fo,fe]
            prefix='/api/v1' if resource=='pods' else '/apis/extensions.agents.x-k8s.io/v1alpha1'
            uri=prefix+'/namespaces/'+args.namespace+'/'+resource+'?watch=true&resourceVersion='+initial['metadata']['resourceVersion']+'&timeoutSeconds=180'
            argv=['kubectl','--context',args.context,'get','--raw',uri]
            proc=subprocess.Popen(argv,stdout=fo,stderr=fe,text=True)
            watch.append((resource,proc))
        # ResourceVersion starts at the successful list, so events between
        # list and watch connection are replayed instead of being missed.
        denied=json.loads(json.dumps(base));denied['metadata']['name']='e15-198-supplement-denied-'+run_id;denied['spec']['projectRef']='billing'
        path=out/'denied.request.json';save(path.name,denied)
        before=count();r=cli(['run','-f',str(path)],'denied.run',check=False)
        time.sleep(3)
        denied_view=capture(denied['metadata']['name'],'denied')
        denial_count={'before':before,'after':count(),'exit':r.returncode}
        denial_count['delta']=denial_count['after']-before
        denial_count['watchEvents']={}
        for resource,proc in watch:
            if proc.poll() is not None:raise RuntimeError('watch exited before window finished')
            ev=events(resource)
            denial_count['watchEvents'][resource]=len(ev)
            save(resource+'.denial-window.events.json',ev)
            save(resource+'.after-denial.json',get(resource))
        save('denial-window.json',denial_count)
        assert denied_view['state']['decision']['result']=='Deny' and denied_view['state'].get('claim') is None
        assert denial_count['delta']==0
        assert all(n==0 for n in denial_count['watchEvents'].values()), 'Resource events during denial require inspection'
        # Verify the scan can detect the exact synthetic marker in the cluster,
        # then remove only this successful create by its returned UID.
        cm={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'e15-198-scan-positive'},'data':{'synthetic':marker}}
        created=json.loads(command(k+['create','-f','-','-o','json'],input=json.dumps(cm)).stdout)
        try:
            control=scan('canary-scan-positive')
            assert control['resources']['configmaps']['matchingObjectNames']==['e15-198-scan-positive']
        finally:
            options={'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':created['metadata']['uid']}}
            command(['kubectl','--context',args.context,'delete','--raw','/api/v1/namespaces/'+args.namespace+'/configmaps/e15-198-scan-positive','-f','-'],input=json.dumps(options))
        canary=json.loads(json.dumps(base));canary['metadata']['name']='e15-198-supplement-canary-'+run_id;canary['spec']['task']['input']['apiKey']=marker
        save('canary.request.json',canary)
        before=count();status,body=api('/api/requests',json.dumps(canary).encode())
        save('canary.api.json',{'status':status,'body':body,'before':before,'after':count()})
        assert status==400 and count()==before
        for scene in ['control','profile']:
            req=json.loads(json.dumps(base));req['metadata']['name']='e15-198-supplement-'+scene+'-'+run_id
            req['spec']['task']['input']['objective']='e15-scenario='+scene
            req['spec']['runtime']['timeout']='10m'
            if scene=='profile':
                req['spec']['requestedAccess']['tools']=['git.read','git.write']
                req['spec']['requestedAccess']['resourceScopes']=['repo:acme/payments']
            path=out/(scene+'.request.json');save(path.name,req)
            before=count();r=cli(['run','-f',str(path)],scene+'.run',check=False)
            v=capture(req['metadata']['name'],scene)
            save(scene+'.count.json',{'before':before,'after':count(),'delta':count()-before,'exit':r.returncode})
            assert v['state']['claim']['phase']==('Succeeded' if scene=='control' else 'Failed')
            assert v['state']['effectiveAuthority']['runtime']['timeout']=='5m0s'
            if scene=='profile':assert v['state']['effectiveAuthority']['tools']==['git.read']
        # Allow warm-pool replenishment before examining agent env/logs.
        deadline=time.monotonic()+30
        while time.monotonic()<deadline:
            pods=get('pods')['items']
            ready=[p for p in pods if any(c['name']=='agent' for c in p['spec']['containers']) and p.get('status',{}).get('phase')=='Running']
            if ready:break
            time.sleep(1)
        final_scan=scan('canary-scan-final')
        assert not any(v['matchingObjectNames'] for v in final_scan['resources'].values())
        assert all(v['readable'] and not v['canaryPresent'] for v in final_scan['logs'].values())
        assert final_scan['environment'] and all(v['readable'] and not v['canaryPresent'] for v in final_scan['environment'].values())
        _,setup_before=api('/api/setup')
        policy={'apiVersion':'agenova.io/v1alpha1','kind':'PolicyBundle','metadata':{'name':'e15-198-constraints-probe','version':'1'},'spec':{'rules':[{'team':'team-a','action':'claim.create','project':'payments','templateRef':'engineer','constraints':{'tools':[]}}]}}
        path=out/'policy-constraints-probe.json';save(path.name,policy)
        r=cli(['policy','apply','-f',str(path)],'policy-constraints-probe',check=False)
        _,setup_after=api('/api/setup')
        assert r.returncode!=0 and 'constraints' in r.stderr
        assert setup_before['policy']==setup_after['policy']
        save('policy-result.json',{'exit':r.returncode,'unsupportedField':'constraints','activePolicyUnchanged':True,'activePolicy':setup_after['policy'],'interpretation':'Policy constraint schema not implemented; template narrowing observed separately'})
    finally:
        for _,proc in watch:
            if proc.poll() is None:
                proc.terminate();proc.wait(timeout=10)
        for h in handles:h.close()
        status=[]
        for resource,proc in watch:
            status.append({'resource':resource,'exit':proc.returncode,'stderr':(out/(resource+'.watch.stderr')).read_text(),'events':len(events(resource))})
        save('watch-status.json',status)
    assert any(e.get('type')=='ADDED' for e in events('pods'))
    assert any(e.get('type')=='ADDED' for e in events('sandboxclaims'))
    save('result.json',{'status':'PASS','context':args.context,'namespace':args.namespace,'startedUnix':start,'finishedUnix':time.time(),'resourceWatchPositiveControl':'allowed Work produced ADDED events','canaryPositiveControl':'exact marker detected then UID-deleted','uiVerification':'separate browser run required','policyNarrowing':'not implemented; unsupported field rejected'})
    print('PASS: denied-allocation watch; watch positive control; canary API+scan; CLI/API parity; policy schema probe',flush=True)


if __name__=='__main__':main()

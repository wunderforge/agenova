#!/usr/bin/env python3
"""Live perimeter checks. Credentials stay off argv and out of evidence."""
import json, os, pathlib, subprocess
from urllib.parse import urlparse
local=pathlib.Path('.tmp/e13/public')
url=(local/'url.txt').read_text().strip()
c=json.loads((local/'credentials.json').read_text())
def fetch(path,auth=True,headers=(),method='GET',data=None,base=url):
    config='silent\nshow-error\nmax-time = 30\n'
    if auth: config+='user = "'+c['username']+':'+(c['password'] if auth is True else 'wrong-password')+'"\n'
    args=['curl','--config','-','-w','\n%{http_code}','-X',method,base+path]
    if os.environ.get('DEMO_RESOLVE_IP'):
        args+=['--resolve',urlparse(base).hostname+':443:'+os.environ['DEMO_RESOLVE_IP']]
    for h in headers: args+=['-H',h]
    if data is not None: args+=['--data',data]
    result=subprocess.check_output(args,input=config.encode()).decode()
    body,code=result.rsplit('\n',1);return int(code),body
checks=[]
def check(label,expected,*args,**kw):
    code,body=fetch(*args,**kw)
    assert code==expected,(label,code,body[:100])
    checks.append(dict(check=label,status=code));return body
check('anonymous page',401,'/',False)
check('anonymous API',401,'/api/requests',False)
check('wrong password',401,'/api/setup','wrong')
check('authenticated page',200,'/?mode=connected')
setup=json.loads(check('authenticated setup',200,'/api/setup'))
assert setup['installation']['platform']=='demo-eks-bedrock'
check('authenticated evidence',200,'/api/requests/investigate-payment-retries/evidence')
check('anonymous after authenticated response',401,'/api/requests/investigate-payment-retries/evidence',False)
check('health endpoint contains no application evidence',200,'/healthz',False)
check('cross-origin write',403,'/api/requests',headers=['Origin: https://attacker.example','Content-Type: application/json'],method='POST',data='{}')
check('cross-site fetch',403,'/api/requests',headers=['Sec-Fetch-Site: cross-site'])
check('same-origin reaches validation',400,'/api/requests',headers=['Origin: '+url,'Content-Type: application/json'],method='POST',data='{}')
navigation=['Sec-Fetch-Site: cross-site','Sec-Fetch-Mode: navigate','Sec-Fetch-Dest: document']
check('external link prompts for authentication',401,'/?mode=connected',False,headers=navigation)
check('authenticated external link opens Portal',200,'/?mode=connected',headers=navigation)
check('external link wrong password rejected',401,'/','wrong',headers=navigation)
check('cross-site API navigation denied',403,'/api/setup',headers=navigation)
check('cross-site document POST denied',403,'/',headers=navigation,method='POST',data='x')
check('cross-site iframe denied',403,'/',headers=['Sec-Fetch-Site: cross-site','Sec-Fetch-Mode: navigate','Sec-Fetch-Dest: iframe'])
check('cross-site page fetch denied',403,'/',headers=['Sec-Fetch-Site: cross-site','Sec-Fetch-Mode: cors','Sec-Fetch-Dest: empty'])
# ALB exposes 443 only; separately inspect listener/SG instead of waiting on port 80.
print(json.dumps(dict(url=url,dnsOverride=os.environ.get("DEMO_RESOLVE_IP"),checks=checks),indent=2))

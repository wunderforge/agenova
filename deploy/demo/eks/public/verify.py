#!/usr/bin/env python3
"""Live perimeter checks. Credentials stay off argv and out of evidence."""
import json, pathlib, subprocess
local=pathlib.Path('.tmp/e13/public')
url=(local/'url.txt').read_text().strip()
c=json.loads((local/'credentials.json').read_text())
def fetch(path,auth=True,headers=(),method='GET',data=None,base=url):
    config='silent\nshow-error\nmax-time = 30\n'
    if auth: config+='user = "'+c['username']+':'+(c['password'] if auth is True else 'wrong-password')+'"\n'
    args=['curl','--config','-','-w','\n%{http_code}','-X',method,base+path]
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
check('cross-origin write',403,'/api/requests',headers=['Origin: https://attacker.example','Content-Type: application/json'],method='POST',data='{}')
check('cross-site fetch',403,'/api/requests',headers=['Sec-Fetch-Site: cross-site'])
check('same-origin reaches validation',400,'/api/requests',headers=['Origin: '+url,'Content-Type: application/json'],method='POST',data='{}')
check('plain HTTP does not challenge for password',403,'/',False,base=url.replace('https:','http:'))
print(json.dumps(dict(url=url,checks=checks),indent=2))

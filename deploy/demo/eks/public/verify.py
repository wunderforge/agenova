#!/usr/bin/env python3
"""Live demo session checks. Never print passwords or session cookies."""
import http.cookiejar, re, json, pathlib, time, urllib.error, urllib.parse, urllib.request
local=pathlib.Path('.tmp/e13/public')
url=(local/'url.txt').read_text().strip().rstrip('/')
c=json.loads((local/'credentials.json').read_text())
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs): return None
jar=http.cookiejar.CookieJar()
client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar),NoRedirect())
anonymous=urllib.request.build_opener(NoRedirect())
checks=[]
def check(label,status,path,method='GET',data=None,headers=None,opener=client):
    if method=='POST' and path in ['/login','/logout'] and data is not None and headers and headers.get('Origin') in [url,'null']:
        form=client.open(url+'/login',timeout=30).read().decode()
        data={**data,'csrf':re.search(r'name="csrf" value="([^"]+)"',form).group(1)}
    req=urllib.request.Request(url+path,data=urllib.parse.urlencode(data).encode() if data is not None else None,method=method,headers=headers or {})
    try: response=opener.open(req,timeout=30)
    except urllib.error.HTTPError as error: response=error
    body=response.read().decode()
    assert response.code==status,(label,response.code,body[:80])
    assert not response.headers.get('WWW-Authenticate'),label
    checks.append(dict(check=label,status=response.code))
    return response,body
nav={'Sec-Fetch-Site':'cross-site','Sec-Fetch-Mode':'navigate','Sec-Fetch-Dest':'document'}
r,_=check('anonymous entry redirects to form',303,'/',opener=anonymous)
assert r.headers['Location']=='/login',r.headers['Location']
check('external link redirects to form',303,'/',headers=nav,opener=anonymous)
check('login form is public',200,'/login',opener=anonymous)
check('anonymous API denied without Basic challenge',401,'/api/setup',opener=anonymous)
check('login CSRF denied',403,'/login','POST',c,{'Origin':'https://attacker.example'})
check('missing login Origin denied',403,'/login','POST',c)
check('wrong password rejected',401,'/login','POST',dict(username=c['username'],password='wrong'),{'Origin':url})
time.sleep(.3)
r,_=check('opaque-origin login with CSRF token creates session',303,'/login','POST',c,{'Origin':'null'})
header=r.headers['Set-Cookie']
assert all(value in header for value in ['Secure','HttpOnly','SameSite=Lax','Path=/','__Host-agenova_demo='])
_,body=check('authenticated Portal',200,'/?mode=connected')
assert 'href="/logout"' in body and '>Sign out</a>' in body
check('authenticated external navigation',200,'/',headers=nav)
_,body=check('authenticated setup',200,'/api/setup');assert json.loads(body)['installation']['platform']=='demo-eks-bedrock'
check('real Work evidence',200,'/api/requests/investigate-payment-retries/evidence')
check('anonymous response isolation',401,'/api/setup',opener=anonymous)
check('tampered session denied',401,'/api/setup',headers={'Cookie':'__Host-agenova_demo=invalid'},opener=anonymous)
check('cross-site API denied',403,'/api/setup',headers=nav)
check('cross-origin write denied',403,'/api/requests','POST',{}, {'Origin':'https://attacker.example'})
check('cross-site iframe denied',403,'/',headers={**nav,'Sec-Fetch-Dest':'iframe'})
check('logout CSRF denied',403,'/logout','POST',{}, {'Origin':'https://attacker.example'})
old_cookie='; '.join(cookie.name+'='+cookie.value for cookie in jar)
check('logout revokes session',303,'/logout','POST',{}, {'Origin':url})
check('revoked cookie denied',401,'/api/setup',headers={'Cookie':old_cookie},opener=anonymous)
check('logged-out API denied',401,'/api/setup')
print(json.dumps(dict(url=url,checks=checks),indent=2))

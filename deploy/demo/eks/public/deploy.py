#!/usr/bin/env python3
"""Explicit ephemeral demo perimeter; run from repository root with EKS env set."""
import base64, hashlib, json, os, pathlib, secrets, subprocess, tarfile

NS = 'agenova-system'
NAME = 'agenova-demo-public'
NGINX = 'nginx@sha256:0985e772fb9f729e6fa0980da05fca5d9c468e870eed43071545afa9d2e27d94'
TUNNEL = 'cloudflare/cloudflared@sha256:072c067d25ccbe61d46e18f0d0723255f2bb5304f7317caa95b27031520ff92c'
def run(args, **kw):
    return subprocess.check_output(args, **kw)
def kube(*args):
    return run(['kubectl', '-n', NS, *args])
def resource(kind, name=NAME, **kw):
    version = 'rbac.authorization.k8s.io/v1' if kind in ('Role','RoleBinding') else 'apps/v1' if kind == 'Deployment' else 'v1'
    return dict(apiVersion=version, kind=kind, metadata=dict(name=name, namespace=NS), **kw)
if os.environ.get('AWS_PROFILE') != 'agenova-demo':
    raise SystemExit('Set explicit AWS_PROFILE=agenova-demo and dedicated KUBECONFIG')
identity=json.loads(run(['aws','sts','get-caller-identity']))
if identity['Account'] != '931228356546' or not identity['Arn'].endswith(':user/agenova-demo-operator'):
    raise SystemExit('Unexpected AWS identity')
if run(['kubectl','config','current-context']).decode().strip() != 'agenova-demo':
    raise SystemExit('Unexpected Kubernetes context')
pods=json.loads(kube('get','pods','-l','app.kubernetes.io/name=agenova-control-plane','-o','json'))['items']
if len(pods)!=1: raise SystemExit('Expected one control-plane Pod')
pod=pods[0]['metadata']['name']
image=pods[0]['spec']['containers'][0]['image']
local=pathlib.Path('.tmp/e13/public'); local.mkdir(parents=True,exist_ok=True); local.chmod(0o700)
credentials=local/'credentials.json'
if not credentials.exists():
    fd=os.open(credentials,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'w') as f: json.dump(dict(username='demo',password=secrets.token_urlsafe(32)),f)
creds=json.loads(credentials.read_text())
verifier=run(['/usr/sbin/htpasswd','-niB',creds['username']],input=(creds['password']+'\n').encode())
with tarfile.open(local/'site.tgz','w:gz') as tar:
    for path in pathlib.Path('ui/dist').iterdir(): tar.add(path,arcname=path.name)
mount=lambda name,path: dict(name=name,mountPath=path,readOnly=True)
security=dict(allowPrivilegeEscalation=False,readOnlyRootFilesystem=True,capabilities=dict(drop=['ALL']))
def container(name,image,command,args,mounts):
    return dict(name=name,image=image,command=command,args=args,securityContext=security,volumeMounts=mounts,resources=dict(requests=dict(cpu='10m',memory='32Mi'),limits=dict(cpu='250m',memory='128Mi')))
objects=[resource('ServiceAccount',automountServiceAccountToken=False),
resource('Role',rules=[dict(apiGroups=[''],resources=['pods'],resourceNames=[pod],verbs=['get']),dict(apiGroups=[''],resources=['pods/portforward'],resourceNames=[pod],verbs=['create'])]),
resource('RoleBinding',subjects=[dict(kind='ServiceAccount',name=NAME,namespace=NS)],roleRef=dict(apiGroup='rbac.authorization.k8s.io',kind='Role',name=NAME)),
resource('Secret',type='Opaque',data={'htpasswd':base64.b64encode(verifier).decode()}),
resource('ConfigMap',NAME+'-config',data={'nginx.conf':pathlib.Path('deploy/demo/eks/public/nginx.conf').read_text()}),
resource('ConfigMap',NAME+'-site',binaryData={'site.tgz':base64.b64encode((local/'site.tgz').read_bytes()).decode()})]
proxy=container('proxy',NGINX,['nginx'],['-c','/config/nginx.conf','-g','daemon off;'],[mount('config','/config'),mount('auth','/auth'),mount('site','/site'),dict(name='tmp',mountPath='/tmp')])
forward=container('forward',image,['/usr/local/bin/kubectl'],['-n',NS,'port-forward','pod/'+pod,'8088:8081','--address=127.0.0.1'],[mount('token','/var/run/secrets/kubernetes.io/serviceaccount')])
tunnel=container('tunnel',TUNNEL,['cloudflared'],['tunnel','--no-autoupdate','--protocol','http2','--metrics','127.0.0.1:20241','--url','http://127.0.0.1:8089'],[])
init=container('unpack',NGINX,['tar'],['xzf','/bundle/site.tgz','-C','/site'],[mount('bundle','/bundle'),dict(name='site',mountPath='/site')])
volumes=[dict(name='config',configMap=dict(name=NAME+'-config')),dict(name='bundle',configMap=dict(name=NAME+'-site')),dict(name='auth',secret=dict(secretName=NAME)),dict(name='site',emptyDir={}),dict(name='tmp',emptyDir={}),dict(name='token',projected=dict(sources=[dict(serviceAccountToken=dict(path='token',expirationSeconds=3600)),dict(configMap=dict(name='kube-root-ca.crt',items=[dict(key='ca.crt',path='ca.crt')])),dict(downwardAPI=dict(items=[dict(path='namespace',fieldRef=dict(fieldPath='metadata.namespace'))]))]))]
objects.append(resource('Deployment',spec=dict(replicas=1,selector=dict(matchLabels=dict(app=NAME)),template=dict(metadata=dict(labels=dict(app=NAME),annotations={'demo.agenova.io/content':hashlib.sha256(b''.join(p.read_bytes() for p in sorted(pathlib.Path('ui/dist').rglob('*')) if p.is_file())+pathlib.Path('deploy/demo/eks/public/nginx.conf').read_bytes()).hexdigest()}),spec=dict(serviceAccountName=NAME,automountServiceAccountToken=False,securityContext=dict(runAsNonRoot=True,runAsUser=65532,runAsGroup=65532,fsGroup=65532,seccompProfile=dict(type='RuntimeDefault')),initContainers=[init],containers=[proxy,forward,tunnel],volumes=volumes)))))
# Never persist the Secret manifest or print credentials.
print(run(['kubectl','apply','-f','-'],input=json.dumps(dict(apiVersion='v1',kind='List',items=objects)).encode()).decode())
print('Credential file:',credentials.resolve())

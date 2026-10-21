#!/usr/bin/env python3
"""Isolated Docker smoke of production nginx. Does not contact AWS or live users."""
import pathlib
import subprocess
import tempfile
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[3]
IMAGE = 'nginx@sha256:0985e772fb9f729e6fa0980da05fca5d9c468e870eed43071545afa9d2e27d94'
name = 'agenova-proxy-test-' + uuid.uuid4().hex[:8]
networks = [name + '-trusted', name + '-untrusted']

def docker(*args, data=None):
    return subprocess.check_output(['docker', *args], input=data, stderr=subprocess.STDOUT).decode()

try:
    with tempfile.TemporaryDirectory() as temporary:
        config = (ROOT / 'deploy/production/nginx.conf').read_text()
        # A same-container echo upstream exposes only synthetic test headers.
        config = config.rsplit('}', 1)[0] + '''
 server { listen 8081; location / { return 200 "$http_x_agenova_client_ip|$http_x_forwarded_for"; } }
}
'''
        path = pathlib.Path(temporary) / 'nginx.conf'
        path.write_text(config)
        docker('network', 'create', '--subnet', '10.82.253.0/24', networks[0])
        docker('network', 'create', '--subnet', '192.0.2.0/24', networks[1])
        docker('run', '-d', '--name', name, '--network', networks[0], '--ip', '10.82.253.2',
               '-v', str(path) + ':/etc/nginx/nginx.conf:ro', IMAGE)
        docker('network', 'connect', '--ip', '192.0.2.2', networks[1], name)
        docker('exec', name, 'nginx', '-t')
        def call(trusted, route='/', payload=None):
            subnet = '10.82.253' if trusted else '192.0.2'
            args = ['run', '--rm', '-i', '--network', networks[0 if trusted else 1],
                    '--ip', subnet+'.3', IMAGE, 'curl', '--max-time', '15', '-sS',
                    '-H', 'Host: app.agenova.app', '-H', 'X-Forwarded-Proto: https',
                    '-H', 'X-Forwarded-For: 203.0.113.99, 198.51.100.7',
                    '-H', 'X-Agenova-Client-IP: 203.0.113.88', '-w', '\n%{http_code}']
            if payload is not None:
                args += ['--data-binary', '@-']
            return docker(*args, 'http://'+subnet+'.2:8089'+route, data=payload)
        assert call(True) == '198.51.100.7|\n200', 'trusted ALB tail/overwrite failed'
        assert call(False) == '192.0.2.3|\n200', 'untrusted forwarded source accepted'
        assert call(True, '/api/governance', b'x' * 200000).endswith('\n200')
        assert call(True, '/api/governance', b'x' * 1052673).endswith('\n413')
        assert call(True, '/login', b'x' * 200000).endswith('\n413')
        print('PASS: trusted/untrusted proxy sources, overwrite, governance and ordinary body limits')
finally:
    subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    for network in networks:
        subprocess.run(['docker', 'network', 'rm', network], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

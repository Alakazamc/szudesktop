"""Run the actual GUI executable with an isolated config and no real authentication."""
import json, os, socket, struct, subprocess, sys, tempfile, time, urllib.request, urllib.error
from pathlib import Path
ROOT=Path(__file__).resolve().parent.parent
EXE=Path(sys.argv[1]) if len(sys.argv)>1 else ROOT/'dist/szudesktop-windows-amd64.exe'
for stream in (sys.stdout,sys.stderr):
    if hasattr(stream,'reconfigure'): stream.reconfigure(encoding='utf-8',errors='replace')
if len(sys.argv)>2: port=int(sys.argv[2])
else:
    with socket.socket() as sock: sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
BASE=f'http://127.0.0.1:{port}'
def request(path,data=None,method=None,headers=None):
    h={'Content-Type':'application/json'} if data is not None else {}
    h.update(headers or {})
    req=urllib.request.Request(BASE+path,data=json.dumps(data).encode() if data is not None else None,method=method,headers=h)
    try:
        with urllib.request.urlopen(req,timeout=45) as r: return r.status,r.read(),r.headers
    except urllib.error.HTTPError as e: return e.code,e.read(),e.headers
def get(path):
    code,body,_=request(path);assert code==200,(path,code,body);return json.loads(body)
def check(name,ok):
    assert ok,name
    print('PASS',name)
raw=EXE.read_bytes();pe=struct.unpack_from('<I',raw,0x3c)[0]
check('Windows GUI subsystem: no console',struct.unpack_from('<H',raw,pe+24+68)[0]==2)
with tempfile.TemporaryDirectory(prefix='szudesktop-smoke-') as cfg:
    proc=subprocess.Popen([str(EXE),'--no-open','--no-auto-login','--addr',f'127.0.0.1:{port}'],env=dict(os.environ,SZUNET_CONFIG_DIR=cfg),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    try:
        for _ in range(40):
            if proc.poll() is not None: raise RuntimeError('test process exited')
            try:
                if request('/api/workspace')[0]==200: break
            except OSError: pass
            time.sleep(.25)
        else: raise RuntimeError('server did not start')
        check('test process is alive',proc.poll() is None)
        for name in ['/api/status','/api/diag','/api/credential','/api/vpn/status','/api/campus/status']:
            check(name,isinstance(get(name),dict))
        check('default VPN unavailable',get('/api/vpn/status')['state']=='unavailable')
        check('no account exposed in status',get('/api/status')['username']=='')
        for path,file in [('/',ROOT/'desktop/index.html'),('/assets/garden/app.mjs',ROOT/'desktop/assets/garden/app.mjs'),('/assets/garden/style.css',ROOT/'desktop/assets/garden/style.css'),('/assets/garden/engine.mjs',ROOT/'desktop/assets/garden/engine.mjs'),('/assets/garden/campus.png',ROOT/'desktop/assets/garden/campus.png'),('/assets/szudesktop.ico',ROOT/'desktop/assets/szudesktop.ico')]:
            code,body,_=request(path);check('embedded '+path,code==200 and body==file.read_bytes())
        check('legacy borrowed art not packaged',request('/assets/art/m1.png')[0]==404)
        credential={'username':'000000','password':'smoke-test-only-not-real'}
        check('save isolated test credential',request('/api/credential',credential)[0]==200)
        check('saved username remains hidden',get('/api/credential')['username']=='')
        check('explicit username reveal',get('/api/credential?reveal=1')['username']=='000000')
        check('credential never returns password','password' not in get('/api/credential?reveal=1'))
        check('delete isolated credential',request('/api/credential',method='DELETE')[0]==200)
        code,body,_=request('/api/login',{'username':'','password':''});check('empty login explains failure',code==200 and not json.loads(body)['ok'])
        w=get('/api/workspace');check('initial workspace empty',w['data'] is None)
        snapshot={'version':1,'revision':0,'data':{'test':'restart'}}
        check('workspace write',request('/api/workspace',snapshot)[0]==200)
        check('stale workspace rejected',request('/api/workspace',snapshot)[0]==409)
        check('cross-origin mutation rejected',request('/api/workspace',snapshot,headers={'Origin':'https://example.com'})[0]==403)
        check('shutdown requires POST',request('/api/shutdown')[0]==405)
        check('application exit endpoint',request('/api/shutdown',{})[0]==200)
        proc.wait(timeout=6);check('clean shutdown',proc.returncode==0)
        # A new random port must see the same file-backed save.
        with socket.socket() as sock: sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
        BASE=f'http://127.0.0.1:{port}'
        proc=subprocess.Popen([str(EXE),'--no-open','--no-auto-login','--addr',f'127.0.0.1:{port}'],env=dict(os.environ,SZUNET_CONFIG_DIR=cfg),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        for _ in range(40):
            try:
                w=get('/api/workspace');break
            except OSError: time.sleep(.25)
        else: raise RuntimeError('restart failed')
        check('save survives process and port change',w['revision']==1 and w['data']=={'test':'restart'})
    finally:
        if proc.poll() is None:
            proc.terminate()
            try: proc.wait(timeout=5)
            except subprocess.TimeoutExpired: proc.kill();proc.wait(timeout=5)
print('ALL SMOKE CHECKS PASSED')

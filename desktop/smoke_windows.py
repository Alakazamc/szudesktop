"""Run the actual GUI executable with an isolated config and no real authentication."""
import http.client, json, os, re, socket, struct, subprocess, sys, tempfile, time
from pathlib import Path
ROOT=Path(__file__).resolve().parent.parent
EXE=Path(sys.argv[1]) if len(sys.argv)>1 else ROOT/'dist/szudesktop-windows-amd64.exe'
for stream in (sys.stdout,sys.stderr):
    if hasattr(stream,'reconfigure'): stream.reconfigure(encoding='utf-8',errors='replace')
if len(sys.argv)>2: port=int(sys.argv[2])
else:
    with socket.socket() as sock: sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
BASE=f'http://127.0.0.1:{port}'
# Use a direct local HTTP connection: no system proxy and no premature
# Connection: close while the server is rejecting an unread request body.
def request(path,data=None,method=None,headers=None):
    h={'Content-Type':'application/json'} if data is not None else {}
    h.update(headers or {})
    body=json.dumps(data).encode() if data is not None else None
    conn=http.client.HTTPConnection('127.0.0.1',port,timeout=45)
    try:
        conn.request(method or ('POST' if data is not None else 'GET'),path,body,headers=h)
        response=conn.getresponse()
        return response.status,response.read(),response.headers
    finally:
        conn.close()

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
        duplicate=subprocess.Popen([str(EXE),'--no-open','--no-auto-login'],env=dict(os.environ,SZUNET_CONFIG_DIR=cfg),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        try:
            duplicate.wait(timeout=10)
            check('duplicate launch reuses instance',duplicate.returncode==0 and proc.poll() is None)
        finally:
            if duplicate.poll() is None: duplicate.kill();duplicate.wait()
        check('instance rejects wrong token',request('/api/instance',{'token':'wrong','open':False})[0]==403)
        check('window API rejects cross origin',request('/api/window',{'id':'smoke-window-primary'},headers={'Origin':'https://example.com'})[0]==403)
        check('window heartbeat accepted',request('/api/window',{'id':'smoke-window-primary'})[0]==200)
        check('second window heartbeat accepted',request('/api/window',{'id':'smoke-window-second'})[0]==200)
        check('second window close accepted',request('/api/window',{'id':'smoke-window-second','closing':True})[0]==200)
        for name in ['/api/status','/api/diag','/api/credential','/api/vpn/status','/api/campus/status']:
            check(name,isinstance(get(name),dict))
        check('notice source is allowlisted',request('/api/campus/notices?source=https://example.com')[0]==400)
        check('notices reject cross origin',request('/api/campus/notices?source=undergrad',headers={'Origin':'https://example.com'})[0]==403)
        check('notices reject POST',request('/api/campus/notices?source=undergrad',{})[0]==405)
        check('bundled official calendar available offline',get('/api/campus/calendar')['terms'][0]['week_start']=='2026-08-30')
        check('calendar rejects cross origin',request('/api/campus/calendar',headers={'Origin':'https://example.com'})[0]==403)
        check('calendar rejects POST',request('/api/campus/calendar',{})[0]==405)
        check('academic login starts signed out',get('/api/academic/session')['authenticated'] is False)
        check('timetable requires academic login',request('/api/academic/timetable')[0]==409)
        check('academic login rejects cross origin',request('/api/academic/login',{},headers={'Origin':'https://example.com'})[0]==403)
        check('academic login requires complete fields',request('/api/academic/login',{})[0]==400)
        check('default VPN unavailable',get('/api/vpn/status')['state']=='unavailable')
        check('no account exposed in status',get('/api/status')['username']=='')
        for path,file in [('/',ROOT/'desktop/index.html'),('/assets/garden/app.mjs',ROOT/'desktop/assets/garden/app.mjs'),('/assets/garden/style.css',ROOT/'desktop/assets/garden/style.css'),('/assets/garden/engine.mjs',ROOT/'desktop/assets/garden/engine.mjs'),('/assets/garden/campus.mjs',ROOT/'desktop/assets/garden/campus.mjs'),('/assets/garden/campus-ui.mjs',ROOT/'desktop/assets/garden/campus-ui.mjs'),('/assets/garden/academic.mjs',ROOT/'desktop/assets/garden/academic.mjs'),('/assets/garden/school.mjs',ROOT/'desktop/assets/garden/school.mjs'),('/assets/garden/network-status.mjs',ROOT/'desktop/assets/garden/network-status.mjs'),('/assets/garden/campus.png',ROOT/'desktop/assets/garden/campus.png'),('/assets/szudesktop.ico',ROOT/'desktop/assets/szudesktop.ico')]:
            code,body,_=request(path);check('embedded '+path,code==200 and body==file.read_bytes())
        code,css,_=request('/assets/fonts/fusion-pixel.css')
        check('pixel font stylesheet packaged',code==200)
        font_paths=re.findall(r'url\(([^)]+\.woff2)\)',css.decode('utf-8'))
        check('pixel font subsets complete',len(font_paths)>0 and all(request('/assets/fonts/'+name)[0]==200 for name in font_paths))
        check('OFL license packaged',b'SIL OPEN FONT LICENSE' in request('/assets/fonts/LICENSE-OFL.txt')[1])
        check('original flora available',request('/assets/garden/flora/tree.png')[0]==200)
        check('legacy borrowed art not packaged',request('/assets/art/m1.png')[0]==404)
        check('old game font not packaged',request('/assets/fonts/svbold.ttf')[0]==404)
        credential={'username':'000000','password':'smoke-test-only-not-real'}
        check('save isolated test credential',request('/api/credential',credential)[0]==200)
        check('saved username remains hidden',get('/api/credential')['username']=='')
        check('explicit username reveal',get('/api/credential?reveal=1')['username']=='000000')
        check('credential never returns password','password' not in get('/api/credential?reveal=1'))
        check('delete isolated credential',request('/api/credential',method='DELETE')[0]==200)
        check('no school session by default',get('/api/session')['saved'] is False)
        check('graduate session check needs saved session',request('/api/session/check?level=graduate',{})[0]==409)
        check('session probe validates level',request('/api/session/check?level=invalid',{})[0]==400)
        check('session rejects cross-origin write',request('/api/session',{'cookie':'test-only=1'},headers={'Origin':'https://example.com'})[0]==403)
        fake_session='session-smoke-only=not-a-real-cookie'
        check('save isolated school session',request('/api/session',{'cookie':fake_session})[0]==200)
        session_status=get('/api/session')
        check('school session never echoed',session_status['saved'] and fake_session not in json.dumps(session_status) and 'cookie' not in session_status)
        session_file=Path(cfg)/'session.json'
        check('school session encrypted on disk',session_file.exists() and fake_session.encode() not in session_file.read_bytes())
        check('delete isolated school session',request('/api/session',method='DELETE')[0]==200 and not session_file.exists())

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
        stream_conn=http.client.HTTPConnection('127.0.0.1',port,timeout=5)
        stream_conn.request('GET','/api/window-stream?id=smoke-window-stream')
        stream=stream_conn.getresponse()
        check('window stream connected',stream.status==200 and stream.readline()==b': alive\n')
        stream.close();stream_conn.close()
        proc.wait(timeout=16)
        check('closing last window exits the process',proc.returncode==0)
    finally:
        if proc.poll() is None:
            proc.terminate()
            try: proc.wait(timeout=5)
            except subprocess.TimeoutExpired: proc.kill();proc.wait(timeout=5)
print('ALL SMOKE CHECKS PASSED')

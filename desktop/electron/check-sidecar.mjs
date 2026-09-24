import assert from 'node:assert/strict';
import {parseListenUrl} from './listen-url.mjs';
import {startSidecar} from './sidecar.mjs';
import {isSafeExternalUrl} from './external-url.mjs';
import {execFileSync} from 'node:child_process';
import http from 'node:http';
import path from 'node:path';
import fs from 'node:fs';
import os from 'node:os';
import {fileURLToPath} from 'node:url';
const here=path.dirname(fileURLToPath(import.meta.url));
const fake=path.join(here,'testdata','fake-sidecar.mjs');
const fixture=path.join(here,'testdata','fake-lifecycle.mjs');
const tests=[];
const alive=pid=>{try{process.kill(pid,0);return true;}catch{return false;}};
function test(name,fn){tests.push({name,fn});}
function killOwnFixture(pid){
  if(!pid||!alive(pid))return;
  try{if(process.platform==='win32')execFileSync('taskkill',['/pid',String(pid),'/T','/F'],{stdio:'ignore',windowsHide:true});else process.kill(pid,'SIGKILL');}catch{}
}
async function withFixtureFiles(fn){
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'szu-sidecar-test-'));
  try{await fn(dir);}finally{
    // Only terminate PIDs recorded by this test's child fixtures.
    for(const name of ['parent.pid','descendant.pid']){const p=path.join(dir,name);if(fs.existsSync(p))killOwnFixture(Number(fs.readFileSync(p,'utf8')));}
    fs.rmSync(dir,{recursive:true,force:true});
  }
}

test('parseListenUrl: 只接受完整协议行与有效端口',()=>{
  assert.equal(parseListenUrl('szuDesktop 已启动: http://127.0.0.1:52344\n'),'http://127.0.0.1:52344');
  assert.equal(parseListenUrl('szuDesktop 已启动: http://localhost:7000\r\n'),'http://127.0.0.1:7000');
  assert.equal(parseListenUrl('szuDesktop 已复用: http://127.0.0.1:7001\n'),'http://127.0.0.1:7001');
  for(const value of ['','正在启动...\n','http://localhost:7000\n','szuDesktop 已启动: http://127.0.0.1:5','szuDesktop 已启动: http://127.0.0.1:0\n','szuDesktop 已启动: http://127.0.0.1:65536\n','szuDesktop 已启动: http://127.0.0.1:80/path\n'])assert.equal(parseListenUrl(value),null,value);
  assert.equal(parseListenUrl('szuDesktop 已启动: http://127.0.0.1:6001\n[自动登录] 成功\n'),'http://127.0.0.1:6001');
});

test('startSidecar: 不存在的引擎返回受控错误',async()=>{
  await assert.rejects(startSidecar({command:path.join(here,'testdata','does-not-exist.exe'),readyTimeoutMs:500}),/ENOENT|启动失败/);
});

test('startSidecar: 拉起健康引擎，释放输出监听，幂等停止',async()=>{
  const handle=await startSidecar({command:process.execPath,args:[fake]});
  try{
    assert.equal(handle.owned,true);
    assert.equal((await fetch(handle.baseUrl+'/api/status')).status,200);
    assert.equal(handle.child.stdout.listenerCount('data'),0);
    assert.equal(handle.child.listenerCount('close'),0);
  }finally{await Promise.all([handle.stop(),handle.stop()]);}
  assert.equal(alive(handle.child.pid),false);
});

test('stop: 优先等待自己引擎的正常 shutdown',async()=>{
  await withFixtureFiles(async dir=>{
    const handle=await startSidecar({command:process.execPath,args:[fixture,'graceful',dir]});
    try{
      await handle.stop();
      assert.equal(fs.readFileSync(path.join(dir,'graceful.marker'),'utf8'),'shutdown accepted');
      assert.equal(handle.child.exitCode,0);
      assert.equal(alive(handle.child.pid),false);
    }finally{await handle.stop();}
  });
});

test('startSidecar: 分块输出端口时等待换行',async()=>{
  const handle=await startSidecar({command:process.execPath,args:[fixture,'chunked']});
  try{assert.equal((await fetch(handle.baseUrl+'/api/status')).status,200);}finally{await handle.stop();}
});

for(const mode of ['unhealthy','hanging'])test('startSidecar: '+mode+' 健康请求有总截止时间且回收进程',async()=>{
  await withFixtureFiles(async dir=>{
    const started=Date.now();
    let watchdog;
    try{
      await assert.rejects(Promise.race([
        startSidecar({command:process.execPath,args:[fixture,mode,dir],healthTimeoutMs:350}),
        new Promise((_,reject)=>{watchdog=setTimeout(()=>reject(new Error('测试看门狗：健康探测没有截止')),5000);}),
      ]),/健康探测超时/);
    }finally{clearTimeout(watchdog);}
    assert.ok(Date.now()-started<4000,'应及时结束启动失败清理');
    assert.equal(alive(Number(fs.readFileSync(path.join(dir,'parent.pid'),'utf8'))),false);
  });
});

test('startSidecar: 已有服务通过明确复用行接管，停止不杀已有引擎',async()=>{
  const server=http.createServer((req,res)=>res.end('{}'));
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const baseUrl='http://127.0.0.1:'+server.address().port;
  try{
    const handle=await startSidecar({command:process.execPath,args:[fixture,'reuse',baseUrl]});
    assert.equal(handle.owned,false);
    assert.equal(handle.baseUrl,baseUrl);
    assert.equal(handle.child.exitCode,0);
    await handle.stop();
    assert.equal((await fetch(baseUrl+'/api/status')).status,200);
  }finally{await new Promise(resolve=>server.close(resolve));}
});

test('startSidecar: 没有就绪行的零退出不是成功复用',async()=>{
  await assert.rejects(startSidecar({command:process.execPath,args:['-e','process.exit(0)']}),/提前退出/);
});

test('startSidecar: 复用启动器异常退出不返回成功',async()=>{
  await assert.rejects(startSidecar({command:process.execPath,args:[fixture,'reuse-failed','http://127.0.0.1:32123']}),/提前退出/);
});

if(process.platform==='win32')test('stop: Windows 等待整棵自己启动的进程树终止',async()=>{
  await withFixtureFiles(async dir=>{
    const handle=await startSidecar({command:process.execPath,args:[fixture,'tree',dir]});
    try{
      const descendantPid=Number(fs.readFileSync(path.join(dir,'descendant.pid'),'utf8'));
      assert.equal(alive(descendantPid),true);
      await handle.stop();
      assert.equal(alive(handle.child.pid),false);
      assert.equal(alive(descendantPid),false,'stop 完成时后代进程仍存活');
    }finally{await handle.stop();}
  });
});

test('isSafeExternalUrl: 放行 http/https，拒绝危险与相对 URI',()=>{
  for(const url of ['http://example.com','https://example.com','HTTPS://Example.COM'])assert.equal(isSafeExternalUrl(url),true);
  for(const url of ['file:///C:/x','search-ms:query','ms-msdt:x','javascript:alert(1)','data:text/html,x','','relative/path','//example.com'])assert.equal(isSafeExternalUrl(url),false);
});

for(const {name,fn} of tests){await fn();console.log('PASS',name);}
console.log(tests.length+' checks passed');

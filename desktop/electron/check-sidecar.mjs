import assert from 'node:assert/strict';
import {parseListenUrl} from './listen-url.mjs';
import {startSidecar} from './sidecar.mjs';
import path from 'node:path';
import fs from 'node:fs';
import os from 'node:os';
import {fileURLToPath} from 'node:url';
const here=path.dirname(fileURLToPath(import.meta.url));
const fake=path.join(here,'testdata','fake-sidecar.mjs');
const fakeUnhealthy=path.join(here,'testdata','fake-unhealthy.mjs');
let checks=0;const queue=[];
function test(name,fn){queue.push((async()=>{await fn();checks++;console.log('PASS',name)})());}

test('parseListenUrl: 解析打印出的监听地址',()=>{
  assert.equal(parseListenUrl('szuDesktop 已启动: http://127.0.0.1:52344\n'),'http://127.0.0.1:52344');
});
test('parseListenUrl: 行还没出现时返回 null',()=>{
  assert.equal(parseListenUrl('正在启动...\n'),null);
  assert.equal(parseListenUrl(''),null);
});
test('parseListenUrl: 忽略之后的自动登录噪声',()=>{
  const out='szuDesktop 已启动: http://127.0.0.1:6001\n[自动登录] 成功\n';
  assert.equal(parseListenUrl(out),'http://127.0.0.1:6001');
});
test('parseListenUrl: localhost 归一到 127.0.0.1',()=>{
  assert.equal(parseListenUrl('http://localhost:7000'),'http://127.0.0.1:7000');
});
test('startSidecar: 拉起替身、解析端口、健康探测通过、可停止',async()=>{
  const handle=await startSidecar({command:process.execPath,args:[fake]});
  try{
    assert.match(handle.baseUrl,/^http:\/\/127\.0\.0\.1:\d+$/);
    const r=await fetch(handle.baseUrl+'/api/status');
    assert.equal(r.status,200);
  } finally { await handle.stop(); }
});
test('startSidecar: 健康探测失败时杀掉子进程，不留孤儿',async()=>{
  const marker=path.join(os.tmpdir(),`szu-fake-unhealthy-exit-${process.pid}-${Date.now()}.marker`);
  const pidFile=marker+'.pid';
  try{
    await assert.rejects(
      startSidecar({
        command:process.execPath,
        args:[fakeUnhealthy],
        env:{...process.env,FAKE_EXIT_MARKER:marker,FAKE_PID_FILE:pidFile},
        healthTimeoutMs:300,
      }),
      /健康探测超时/
    );
    // 启动失败必须回收子进程：POSIX 上子进程退出会写 marker；
    // Windows 上 SIGKILL/taskkill /F 不跑用户代码（marker 不会写出来），改用 PID 存活探测。
    const childPid=Number(fs.readFileSync(pidFile,'utf8'));
    const alive=()=>{try{process.kill(childPid,0);return true;}catch{return false;}};
    const deadline=Date.now()+3000;
    let killed=false;
    while(Date.now()<deadline){
      if(fs.existsSync(marker)||!alive()){killed=true;break;}
      await new Promise(r=>setTimeout(r,100));
    }
    assert.ok(killed,`失败启动后子进程仍存活（孤儿 sidecar），pid=${childPid}`);
  } finally {
    try{fs.unlinkSync(marker);}catch{}
    try{fs.unlinkSync(pidFile);}catch{}
  }
});

// 顶层 await：node 直接跑 .mjs 支持
await Promise.all(queue);
console.log(checks+' checks passed');

import assert from 'node:assert/strict';
import {parseListenUrl} from './listen-url.mjs';
import {startSidecar} from './sidecar.mjs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const here=path.dirname(fileURLToPath(import.meta.url));
const fake=path.join(here,'testdata','fake-sidecar.mjs');
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

// 顶层 await：node 直接跑 .mjs 支持
await Promise.all(queue);
console.log(checks+' checks passed');

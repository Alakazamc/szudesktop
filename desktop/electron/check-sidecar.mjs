import assert from 'node:assert/strict';
import {parseListenUrl} from './listen-url.mjs';
let checks=0;function test(name,fn){fn();checks++;console.log('PASS',name)}

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

console.log(checks+' checks passed');

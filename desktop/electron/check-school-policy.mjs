import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {academicCookies,isSchoolURL,schoolTargets} from './school-policy.mjs';
for(const url of Object.values(schoolTargets))assert.ok(isSchoolURL(url));
for(const url of ['https://szu.edu.cn.evil.test/','http://ehall.szu.edu.cn/','https://ehall.szu.edu.cn:444/','https://user@ehall.szu.edu.cn/','file:///C:/test','javascript:alert(1)'])assert.equal(isSchoolURL(url),false,url);
assert.deepEqual(academicCookies([
 {domain:'ehall.szu.edu.cn',path:'/jwapp',name:'s',value:'test-only'},
 {domain:'swzx.webvpn.szu.edu.cn',path:'/',name:'vpn',value:'separate'},
 {domain:'example.org',path:'/',name:'other',value:'excluded'},
]),[{name:'s',value:'test-only',path:'/jwapp'}]);
const window=readFileSync(new URL('./school-window.mjs',import.meta.url),'utf8');
assert.match(window,/fromPartition\('szu-official',\{cache:false\}\)/);
assert.match(window,/contextIsolation:true,nodeIntegration:false,sandbox:true/);
assert.doesNotMatch(window,/webSecurity:false|preload:/);
const packaged=readFileSync(new URL('./electron-builder.yml',import.meta.url),'utf8');
for(const name of ['pet-settings.mjs','smoke-pet.mjs','school-window.mjs','school-policy.mjs'])assert.ok(packaged.includes(`- ${name}`),`${name} absent from installer`);
console.log('School window: URL boundary, cookie scope, isolated profile and packaged modules passed');

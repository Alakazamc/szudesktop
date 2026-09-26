import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {academicCookies,isSchoolURL,schoolTargets} from './school-policy.mjs';
for(const url of Object.values(schoolTargets))assert.ok(isSchoolURL(url));
assert.ok(isSchoolURL('https://authserver-443.webvpn.szu.edu.cn/authserver/login?service=https%3A%2F%2Fwebvpn.szu.edu.cn%2F'));
for(const url of ['https://szu.edu.cn.evil.test/','http://ehall.szu.edu.cn/','https://ehall.szu.edu.cn:444/','https://user@ehall.szu.edu.cn/','https://authserver-443.webvpn.szu.edu.cn.evil.test/','http://authserver-443.webvpn.szu.edu.cn/','https://other.webvpn.szu.edu.cn/','file:///C:/test','javascript:alert(1)'])assert.equal(isSchoolURL(url),false,url);
assert.deepEqual(academicCookies([
 {domain:'ehall.szu.edu.cn',path:'/jwapp',name:'s',value:'test-only'},
 {domain:'swzx.webvpn.szu.edu.cn',path:'/',name:'vpn',value:'separate'},
 {domain:'example.org',path:'/',name:'other',value:'excluded'},
]),[{name:'s',value:'test-only',path:'/jwapp'}]);
const window=readFileSync(new URL('./school-window.mjs',import.meta.url),'utf8');
assert.match(window,/fromPartition\('szu-official',\{cache:false\}\)/);
assert.match(window,/contextIsolation:true,nodeIntegration:false,sandbox:true/);
assert.doesNotMatch(window,/webSecurity:false|preload:/);
// Exercise the real sync path without Electron, the network or any user profile.
// An empty official profile must still reach Go so it can replace the selected
// account, rather than leave a previously imported person's session available.
const requests=[];
let responseStatus=400,responseMessage='请先在应用内的学校页面完成登录';
const profile={setPermissionRequestHandler(){},setPermissionCheckHandler(){},cookies:{get:async()=>[]}};
const createSchoolWindows=new Function('session','fetch','academicCookies','isSchoolURL','schoolTargets',
 window.replace(/^import[^\n]*\n/gm,'').replace('export function createSchoolWindows','function createSchoolWindows')+'\nreturn createSchoolWindows;'
)({fromPartition:()=>profile},async(url,options)=>{
 requests.push({url,options});
 return {ok:false,status:responseStatus,json:async()=>({ok:false,message:responseMessage})};
},academicCookies,isSchoolURL,schoolTargets);
const school=createSchoolWindows(()=>'http://127.0.0.1:1234');
for(const business of ['undergrad','graduate','undergrad-scores','graduate-scores']){
 await assert.rejects(school.sync(business),/请先在应用内的学校页面完成登录/);
 const {url,options}=requests.at(-1);
 assert.equal(url,'http://127.0.0.1:1234/api/academic/browser-session');
 assert.equal(options.method,'POST');
 assert.deepEqual(JSON.parse(options.body),{business,cookies:[]});
}
assert.equal(requests.length,4);
responseStatus=403;
responseMessage='当前账号没有所选业务的访问权限，请核对培养层次或在官方系统确认权限';
await assert.rejects(school.sync('undergrad'),{message:responseMessage});
const packaged=readFileSync(new URL('./electron-builder.yml',import.meta.url),'utf8');
for(const name of ['pet-settings.mjs','smoke-pet.mjs','school-window.mjs','school-policy.mjs'])assert.ok(packaged.includes(`- ${name}`),`${name} absent from installer`);
console.log('School window: URL boundary, cookie scope, empty-session replacement, isolated profile and packaged modules passed');

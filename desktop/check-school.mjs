import assert from 'node:assert/strict';
import {createSchoolUI,timetableHTML} from './assets/garden/school.mjs';
const nodes=new Map(),node=id=>{if(!nodes.has(id))nodes.set(id,{innerHTML:'',value:'',disabled:false,reset(){this.value=''}});return nodes.get(id)};
globalThis.document={getElementById:node,querySelector:node};
const sample={entries:[{name:'<img src=x>',day:2,start:3,end:4,weeks:'1–16周（单）',room:'测试教室'}],unscheduled:[{name:'待排课程'}],term:'测试学期',fetched_at:'2026-09-20T00:00:00Z'};
let fail=false,calls=[],count=0;
const ui=createSchoolUI({toast(){},api:async(path,data,method)=>{calls.push({path,method});if(path.endsWith('/session'))return {authenticated:method!=='DELETE'};if(path.endsWith('/challenge'))return {challenge:'fake',image:'/api/academic/captcha?id=fake'};if(path.endsWith('/login'))return {authenticated:true};if(fail)throw Object.assign(Error('登录状态已失效'),{code:401});return sample}});
async function check(name,f){await f();count++;console.log('PASS',name)}
await check('official periods, weeks and unarranged courses are preserved and escaped',()=>{
 const text=timetableHTML(sample);assert.match(text,/&lt;img src=x&gt;/);assert.doesNotMatch(text,/<img src=x>/);assert.match(text,/第 3–4 节/);assert.match(text,/1–16周（单）/);assert.match(text,/待排课程/);
});
await check('official empty timetable is distinct from not queried',()=>{
 assert.match(timetableHTML(null),/登录后点击/);assert.match(timetableHTML({...sample,entries:[],unscheduled:[]}),/学校当前学期没有返回已排定/);
});
await check('relogin clears previously displayed personal schedule',async()=>{
 await ui.load();await ui.click('school-read');assert.match(node('school-timetable').innerHTML,/&lt;img/);
 await ui.click('school-challenge');assert.doesNotMatch(node('school-timetable').innerHTML,/&lt;img/);assert.equal(node('#school-login-form button[type=submit]').disabled,false);
});
await check('login clears password and consumes the image challenge',async()=>{
 const values={username:'test-only',password:'fake-password',captcha:'0000'};node('school-password').value=values.password;
 await ui.submit({id:'school-login-form',reset(){}},values);assert.equal(values.password,'');assert.equal(node('school-password').value,'');assert.equal(node('#school-login-form button[type=submit]').disabled,true);assert.equal(node('[data-action="school-read"]').disabled,false);
});
await check('expired session removes old schedule and disables read',async()=>{
 await ui.click('school-read');fail=true;await ui.click('school-read');assert.match(node('school-status').innerHTML,/已失效/);assert.doesNotMatch(node('school-timetable').innerHTML,/&lt;img/);assert.equal(node('[data-action="school-read"]').disabled,true);
});
console.log(`${count} school checks passed`);

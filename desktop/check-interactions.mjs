import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const source=readFileSync(new URL('./assets/garden/app.mjs',import.meta.url),'utf8');
const start=source.indexOf('async function run(work)'),end=source.indexOf("document.addEventListener('submit'",start);
assert.ok(start>=0&&end>start);
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
function fixture(){
 const calls=[],toasts=[],jobs=new Map(),controls=[{disabled:false,isConnected:true}];let click;
 const context=vm.createContext({
  busy:false,page:'services',state:null,
  document:{querySelectorAll:()=>controls,addEventListener:(_,fn)=>{click=fn}},
  schoolUI:{click:async()=>false,sync:()=>{}},
  campusUI:{click:async action=>{calls.push(action);if(jobs.has(action))await jobs.get(action).promise;return true}},
  pianoUI:{click:async()=>false},
  academicUI:{load:async()=>{}},toast:message=>toasts.push(message),clocks:()=>{},
  navigate:p=>{context.page=p},networkResult:()=>{},
 });
 vm.runInContext(source.slice(start,end),context);
 return {context,calls,toasts,controls,jobs,click(action,extra={}){click({target:{closest:()=>({dataset:{action,...extra}})},preventDefault(){}})}};
}
let count=0;async function check(name,fn){await fn();count++;console.log('PASS',name)}
await check('a pending public venue query allows navigation and a separate write',async()=>{
 const f=fixture(),pending=deferred();f.jobs.set('booking-rooms',pending);
 f.click('booking-rooms');await tick();
 f.click('navigate',{page:'garden'});
 assert.equal(f.context.page,'garden','场地请求等待期间必须允许切页');
 assert.equal(f.controls[0].disabled,false,'公共查询不应禁用其他卡片');
 f.click('campus-reminder-delete');await tick();
 assert.ok(f.calls.includes('campus-reminder-delete'),'只读请求不占用存档写锁');
 pending.resolve();await tick();
});
await check('write actions still serialize and public action names cannot bypass that lock',async()=>{
 const f=fixture(),pending=deferred();f.jobs.set('campus-session-save',pending);
 f.click('campus-session-save');await tick();
 f.click('campus-session-clear');f.click('booking-rooms');f.click('navigate',{page:'home'});await tick();
 assert.deepEqual(f.calls,['campus-session-save']);assert.equal(f.context.page,'services');
 assert.equal(f.context.busy,true);pending.resolve();await tick();assert.equal(f.context.busy,false);
});
await check('public lookup errors do not prevent the next action',async()=>{
 const f=fixture();f.context.campusUI.click=async()=>{throw Error('学校暂时无响应')};
 f.click('notice-read');await tick();assert.deepEqual(f.toasts,['学校暂时无响应']);
 f.click('navigate',{page:'home'});assert.equal(f.context.page,'home');assert.equal(f.context.busy,false);
});
await check('Electron exit uses its shell bridge and never shuts down a shared Go service',async()=>{
 const f=fixture();let quits=0,shutdowns=0;
 f.context.campusUI.click=async()=>false;f.context.confirm=async()=>true;
 f.context.szuDesktop={shell:'electron',quit:async()=>{quits++}};
 f.context.api=async()=>{shutdowns++};
 f.click('shutdown');await tick();
 assert.equal(quits,1);assert.equal(shutdowns,0);assert.deepEqual(f.toasts,[]);
});
await check('portable exit keeps the existing service shutdown behavior',async()=>{
 const f=fixture(),paths=[];
 f.context.campusUI.click=async()=>false;f.context.confirm=async()=>true;
 f.context.api=async path=>{paths.push(path)};f.context.$=()=>({});
 f.context.windowStream=null;f.context.clearInterval=()=>{};
 f.context.clockInterval=1;f.context.networkInterval=2;f.context.calendarInterval=3;
 f.context.window={close(){}};f.context.exiting=false;
 f.click('shutdown');await tick();
 assert.deepEqual(paths,['/api/shutdown']);assert.equal(f.context.exiting,true);assert.deepEqual(f.toasts,[]);
});
await check('onboarding and settings explain the active shell exit behavior',()=>{
 const from=source.indexOf('function exitHint(){'),to=source.indexOf('async function markOnboarded()',from);
 assert.ok(from>=0&&to>from);const context=vm.createContext({});vm.runInContext(source.slice(from,to),context);
 assert.match(vm.runInContext('exitHint()',context),/10 秒/);
 context.szuDesktop={shell:'electron'};
 const installed=vm.runInContext('exitHint()',context);assert.match(installed,/关闭主窗口/);assert.doesNotMatch(installed,/10 秒/);
 const guide=source.slice(source.indexOf('function showGuide(){'),from);
 assert.match(guide,/hint.textContent=exitHint\(\)/);
 const settings=source.slice(source.indexOf('function settings(){'),source.indexOf('function render(){'));
 assert.ok(settings.includes('${exitHint()}'));
});
await check('load failure retry works without an inline script under Electron CSP',()=>{
 const f=fixture();let reloads=0;f.context.location={reload(){reloads++}};
 f.click('reload');assert.equal(reloads,1);
 assert.doesNotMatch(source,/onclick=/);assert.match(source,/data-action="reload"/);
});
console.log(`${count} interaction checks passed`);

import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const source=readFileSync(new URL('./assets/garden/app.mjs',import.meta.url),'utf8');
const start=source.indexOf('async function run('),end=source.indexOf("document.addEventListener('submit'",start);
assert.ok(start>=0&&end>start);
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
function fixture(){
 const calls=[],toasts=[],jobs=new Map(),controls=[{disabled:false,isConnected:true}];let click;
 const context=vm.createContext({
  busy:false,page:'services',state:null,
  document:{querySelectorAll:()=>controls,addEventListener:(_,fn)=>{click=fn}},
  schoolUI:{click:async()=>false,sync:()=>{}},
  officialUI:{click:async()=>false},
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
 const installed=vm.runInContext('exitHint()',context);assert.match(installed,/关闭主窗口/);assert.match(installed,/常驻/);assert.match(installed,/托盘/);assert.doesNotMatch(installed,/10 秒/);
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
await check('the pet size slider only exists under the Electron shell and round-trips through the bridge',()=>{
 const from=source.indexOf('function settings(){'),to=source.indexOf('function render(){',from);
 assert.ok(from>=0&&to>from);
 const settings=source.slice(from,to);
 // 浏览器模式（无 szuDesktop）绝不渲染滑杆。
 assert.match(settings,/globalThis\.szuDesktop\?\.shell==='electron'/);
 assert.match(settings,/id="pet-scale"/);
 assert.match(settings,/id="pet-scale-value"/);
 assert.match(settings,/type="range"/);
 assert.match(settings,/min="0\.4"/);
 assert.match(settings,/max="2"/);
 // 滑杆的读写必须落在桥接函数上，而不是自己写一份状态。
 const wire=source.slice(source.indexOf('function loadPetScale(){'),source.indexOf('function render(){'));
 assert.match(wire,/petScale\(\)\.then/,'必须先从主进程读回当前值');
 assert.match(wire,/setPetScale\(Number\(input\.value\)\)/,'必须把显示值交给桥接函数');
 assert.match(wire,/szuDesktop\?\.setPetScale/,'浏览器模式下不得接线');
});
await check('the unified-auth login lives in the app and never keeps the password',()=>{
 const ui=readFileSync(new URL('./assets/garden/campus-ui.mjs',import.meta.url),'utf8');
 // 表单与三个端点都要在。
 assert.match(ui,/id="cas-login-form"/);
 assert.match(ui,/id="cas-username"/);
 assert.match(ui,/id="cas-password"/);
 assert.match(ui,/\/api\/cas\/challenge/);
 assert.match(ui,/\/api\/cas\/login/);
 assert.match(ui,/\/api\/cas\/session/);
 // 密码用完必须清掉输入框和 values，不能留在页面上。
 assert.match(ui,/values\.password=''/);
 assert.match(ui,/pwd\.value=''/);
 // 明文字密码绝不能进日志或 URL。
 assert.doesNotMatch(ui,/console\.log\([^)]*password/i);
 assert.doesNotMatch(ui,/toast\([^)]*password/i);
 // 会话失效时要同时刷新两条入口的状态，不能只刷新粘 Cookie 那个。
 assert.match(ui,/if\(e\.code===401\)\{sessionErr=e\.message;casLogged=false;/);
 // 登出 CAS 后要能回落到 Cookie，所以不能让cas-clear 顺手把 Cookie 也删了。
 assert.match(ui,/a==='cas-clear'[\s\S]{0,400}?\/api\/cas\/session/);
 assert.doesNotMatch(ui.slice(ui.indexOf("a==='cas-clear'"),ui.indexOf("a==='online-score'")),/\/api\/session/,{},'DELETE');
});
console.log(`${count} interaction checks passed`);

import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import {CROPS,createState,level,normalize} from './assets/garden/engine.mjs';

// Exercise the actual page handlers with an isolated DOM and workspace API.
const source=readFileSync(new URL('./assets/garden/app.mjs',import.meta.url),'utf8');
function section(start,end){
 const from=source.indexOf(start),to=source.indexOf(end,from);
 assert.ok(from>=0&&to>from,'page handler was not found');
 return source.slice(from,to);
}
let checks=0;
async function check(name,fn){await fn();checks++;console.log('PASS',name)}

await check('changing crop updates empty plots without leaving the farm',()=>{
 const handlers={},state=createState();state.game.seeds.radish=0;
 let html='';
 const context=vm.createContext({
  state,CROPS,level,Date,selectedCrop:'radish',busy:false,
  document:{addEventListener:(name,handler)=>{handlers[name]=handler}},
  btn:(text,action,extra='')=>`<button data-action="${action}" ${extra}>${text}</button>`,
  cropIcon:()=>'',plotButton:()=>'',
  render:()=>{html=vm.runInContext('farm(state.game)',context)},
 });
 vm.runInContext(section('function farm(','function plotButton('),context);
 vm.runInContext(section("document.addEventListener('change'",'let exiting='),context);
 context.render();assert.doesNotMatch(html,/data-action="plant"/);
 handlers.change({target:{id:'seed-choice',value:'strawberry'}});
 assert.match(html,/data-action="plant"/);
 handlers.change({target:{id:'seed-choice',value:'radish'}});
 assert.doesNotMatch(html,/data-action="plant"/);
 assert.match(html,/种子不足/);
});

function exportFixture(api){
 let click,pending,blob;
 const local=createState();
 const context=vm.createContext({
  state:local,revision:1,createState,normalize,Blob,Date,
  api,toast:()=>{},setTimeout:()=>{},render:()=>{},
  schoolUI:{click:async()=>false},campusUI:{click:async()=>false},run:work=>{pending=work()},
  document:{addEventListener:(_,handler)=>{click=handler},createElement:()=>({click:()=>{}})},
  URL:{createObjectURL:value=>{blob=value;return 'blob:workspace-test'},revokeObjectURL:()=>{}},
 });
 vm.runInContext(section("document.addEventListener('click'","document.addEventListener('submit'"),context);
 return {
  async download(){
   click({target:{closest:()=>({dataset:{action:'export'}})},preventDefault:()=>{}});
   await pending;
   return JSON.parse(await blob.text());
  },
  hasDownload:()=>!!blob,
 };
}
await check('backup includes the latest save from another window',async()=>{
 const latest=createState();latest.todos.push({id:'new-task',text:'另一窗口已保存的待办',done:false,rewarded:false});
 let reads=0;
 const fixture=exportFixture(async path=>{assert.equal(path,'/api/workspace');reads++;return {version:1,revision:2,data:latest}});
 const backup=await fixture.download();
 assert.equal(reads,1);assert.deepEqual(backup.todos,latest.todos);
});
await check('failed workspace reads do not silently export stale data',async()=>{
 const fixture=exportFixture(async()=>{throw Error('workspace unavailable')});
 await assert.rejects(()=>fixture.download(),/workspace unavailable/);
 assert.equal(fixture.hasDownload(),false);
});
await check('onboarding flag is explicit and survives a save round trip',()=>{
 const fresh=createState();
 assert.equal(fresh.preferences.onboarded,false,'新存档必须还没看过引导');
 const round=JSON.parse(JSON.stringify(fresh));round.preferences.onboarded=true;
 assert.equal(normalize(round,Date.now()).preferences.onboarded,true);
 const junk=JSON.parse(JSON.stringify(fresh));junk.preferences.onboarded='yes';
 assert.equal(normalize(junk,Date.now()).preferences.onboarded,false,'truthy 字符串不能当成已看过引导');
});

await check('dismissing the guide records it once and keeps other preferences',async()=>{
 const local=createState();local.preferences.theme='night';local.preferences.motion=false;
 const committed=[],toasts=[];
 const context=vm.createContext({state:local,structuredClone,toast:m=>toasts.push(m)});
 context.commit=async next=>{committed.push(next);context.state=next};
 vm.runInContext(section('function showGuide(','function todoHTML('),context);
 await vm.runInContext('markOnboarded()',context);
 assert.equal(committed.length,1);
 assert.equal(committed[0].preferences.onboarded,true);
 assert.equal(committed[0].preferences.theme,'night','记录引导状态不能顺手改掉用户选的庭院光线');
 assert.equal(committed[0].preferences.motion,false);
 assert.equal(toasts.length,0);
 await vm.runInContext('markOnboarded()',context);
 assert.equal(committed.length,1,'已经看过引导就不该再写一次存档');
});

await check('a failed onboarding save is reported, not swallowed',async()=>{
 const local=createState();const toasts=[];
 const context=vm.createContext({state:local,structuredClone,toast:m=>toasts.push(m),commit:async()=>{throw Error('另一个窗口更新了存档')}});
 vm.runInContext(section('function showGuide(','function todoHTML('),context);
 await vm.runInContext('markOnboarded()',context);
 assert.equal(toasts.length,1,'存档没写成功却不告诉用户，下次打开会莫名再弹一次');
 assert.match(toasts[0],/另一个窗口更新了存档/);
});

await check('first run opens the guide, settings save keeps the flag',()=>{
 assert.match(source,/if\(!state\.preferences\.onboarded\)showGuide\(\)/,'启动时没有按存档状态决定是否展示引导');
 assert.match(source,/next\.preferences=\{\.\.\.next\.preferences,theme:d\.theme,motion:!!d\.motion\}/,'保存设置会把「已看过引导」丢掉，用户每次打开都会被再教一遍');
 const html=readFileSync(new URL('./index.html',import.meta.url),'utf8');
 assert.match(html,/<dialog id="guide"><form method="dialog">/);
 assert.match(html,/数据只在本机/);
 assert.match(html,/Esc 关掉/,'必须写明可以跳过，否则用户以为每次打开都会弹');
});
console.log(`${checks} workspace UI checks passed`);

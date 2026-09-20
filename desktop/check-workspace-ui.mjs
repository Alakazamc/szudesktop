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
console.log(`${checks} workspace UI checks passed`);

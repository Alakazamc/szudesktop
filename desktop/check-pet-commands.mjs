import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import {act,activePet,createState,normalize} from './assets/garden/engine.mjs';

const source=readFileSync(new URL('./assets/garden/app.mjs',import.meta.url),'utf8');
const handlers=source.slice(source.indexOf('async function run('),source.indexOf("document.addEventListener('click'"));
const commitSource=source.slice(source.indexOf('async function commit('),source.indexOf('async function confirm('));
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
function fixture(){
 const results=[],toasts=[],writes=[],controls=[{disabled:false,isConnected:true}];
 const context=vm.createContext({
  state:createState(),revision:1,workspaceReady:true,busy:false,exiting:false,page:'home',gardenTab:'pet',
  act,activePet,normalize,render(){},clocks(){},schoolUI:{sync(){}},
  document:{querySelectorAll:selector=>selector==='#main form[id]'?[]:controls,activeElement:null,getElementById:()=>null},toast:message=>toasts.push(message),networkResult(){},
  navigate:page=>{context.page=page},
  szuDesktop:{petResult:result=>results.push({...result})},
  api:async(path,body)=>{assert.equal(path,'/api/workspace');writes.push(body);return {revision:context.revision+1}},
 });
 vm.runInContext(commitSource+handlers,context);
 return {context,results,toasts,writes,controls,command:command=>context.handlePetCommand(command)};
}
let checks=0;
async function check(name,work){await work();checks++;console.log('PASS',name)}
await check('care commands save through the shared engine before reporting success',async()=>{
 const f=fixture(),initial=f.context.state,gate=deferred();
 const save=f.context.api;f.context.api=async(...args)=>{await gate.promise;return save(...args)};
 const pending=f.command('feed');
 assert.equal(f.context.busy,true);assert.equal(f.results.length,0);assert.equal(f.context.state,initial);
 gate.resolve();await pending;
 assert.equal(f.context.state.game.food,initial.game.food-1);
 assert.ok(activePet(f.context.state.game).hunger>activePet(initial.game).hunger);
 assert.equal(f.writes[0].revision,1);assert.equal(f.context.revision,2);
 assert.deepEqual(f.results,[{ok:true,message:'吃饱啦，谢谢你'}]);
 assert.equal(f.context.busy,false);assert.equal(f.controls[0].disabled,false);
 for(const command of ['pat','play','sleep','sleep'])await f.command(command);
 assert.equal(f.writes.length,5);assert.ok(f.results.every(r=>r.ok));
 assert.equal(activePet(f.context.state.game).sleeping,false);
 assert.match(f.results.at(-2).message,/晚安/);assert.match(f.results.at(-1).message,/醒来/);
});
await check('cooldown and unavailable food are real failures without extra writes',async()=>{
 const f=fixture();await f.command('pat');await f.command('pat');
 assert.equal(f.writes.length,1);assert.equal(f.results.at(-1).ok,false);assert.match(f.results.at(-1).message,/10 秒/);
 f.context.state.game.food=0;await f.command('feed');
 assert.equal(f.writes.length,1);assert.equal(f.results.at(-1).ok,false);
});
await check('failed saves keep the original state and send an error instead of a success bubble',async()=>{
 const f=fixture(),original=f.context.state;
 f.context.api=async()=>{throw Error('磁盘暂时无法写入')};await f.command('feed');
 assert.equal(f.context.state,original);assert.equal(f.context.revision,1);
 assert.deepEqual(f.results,[{ok:false,message:'磁盘暂时无法写入'}]);
 assert.deepEqual(f.toasts,['磁盘暂时无法写入']);assert.equal(f.context.busy,false);
});
await check('a conflicting save refreshes the latest record without pretending to apply care',async()=>{
 const f=fixture(),latest=createState();latest.game.food=8;
 f.context.api=async(_path,body)=>{if(body)throw Object.assign(Error('conflict'),{code:409});return {revision:4,data:latest}};
 await f.command('feed');assert.equal(f.context.revision,4);assert.equal(f.context.state.game.food,8);
 assert.equal(f.results[0].ok,false);assert.match(f.results[0].message,/尚未保存/);
});
await check('busy, startup, and shutdown reject commands explicitly',async()=>{
 for(const changed of [{busy:true},{workspaceReady:false},{state:null},{exiting:true}]){
  const f=fixture();Object.assign(f.context,changed);await f.command('feed');
  assert.equal(f.writes.length,0);assert.equal(f.results.length,1);assert.equal(f.results[0].ok,false);
 }
});
await check('navigation selects the requested real page and the correct garden section',async()=>{
 const f=fixture();await f.command('farm');assert.equal(f.context.page,'garden');assert.equal(f.context.gardenTab,'farm');
 await f.command('garden');assert.equal(f.context.page,'garden');assert.equal(f.context.gardenTab,'pet');
 await f.command('study');assert.equal(f.context.page,'study');await f.command('home');assert.equal(f.context.page,'home');
 assert.equal(f.writes.length,0);assert.equal(f.results.length,4);assert.ok(f.results.every(r=>r.ok));
});
await check('pet selection saves the active companion and refuses invalid or stale choices',async()=>{
 const f=fixture(),before=f.context.state.game.pets[0];
 await f.command('switchPet:1');
 assert.equal(f.context.state.game.active,1);assert.equal(f.results[0].ok,true);
 assert.equal(f.context.state.game.pets[0].xp,before.xp);assert.equal(f.writes.length,1);
 assert.match(f.results[0].message,/来陪你/);
 for(const command of ['switchPet:1','switchPet:7','switchPet:8','switchPet:-1','switchPet:0.5','switchPet:constructor'])await f.command(command);
 assert.equal(f.writes.length,1);assert.ok(f.results.slice(1).every(result=>!result.ok));
});
await check('pet selection refreshes the name field instead of keeping the previous companion draft',async()=>{
 const f=fixture();let replaced=false;
 const oldForm={id:'pet-name-form'},newForm={replaceWith(){replaced=true}};
 f.context.document.querySelectorAll=selector=>selector==='#main form[id]'?[oldForm]:f.controls;
 f.context.document.getElementById=()=>newForm;
 await f.command('switchPet:1');assert.equal(f.results[0].ok,true);assert.equal(replaced,false);
});
await check('a care conflict that changes companion cannot attach the previous name draft',async()=>{
 const f=fixture(),latest=createState();latest.game.active=2;let restored=false;
 f.context.page='garden';
 f.context.document.querySelectorAll=selector=>selector==='#main form[id]'?[{id:'pet-name-form'}]:f.controls;
 f.context.document.getElementById=()=>({replaceWith(){restored=true}});
 f.context.api=async(_path,body)=>{if(body)throw Object.assign(Error('conflict'),{code:409});return {revision:5,data:latest}};
 await f.command('feed');
 assert.equal(f.context.state.game.active,2);assert.equal(restored,false);assert.equal(f.results[0].ok,false);
});
await check('the command boundary cannot trigger arbitrary game actions or inherited property names',async()=>{
 const f=fixture();for(const command of ['gift','shutdown','constructor','__proto__',null])await f.command(command);
 assert.equal(f.writes.length,0);assert.equal(f.results.length,5);assert.ok(f.results.every(r=>!r.ok));
});
await check('care leaves unrelated pages and their unfinished inputs in place',async()=>{
 for(const page of ['settings','network','services','study']){
  const f=fixture(),field={value:'未提交内容',isConnected:true,selectionStart:2,selectionEnd:3};
  f.context.page=page;f.context.document.activeElement=field;
  f.context.render=()=>{throw Error('不应重绘与照料无关的页面')};
  await f.command('feed');
  assert.equal(f.results[0].ok,true);assert.equal(f.context.document.activeElement,field);
  assert.equal(field.value,'未提交内容');assert.equal(field.selectionStart,2);assert.equal(field.selectionEnd,3);
 }
});
await check('care keeps the original form node, latest draft, focus, and selection across a repaint',async()=>{
 for(const conflict of [false,true]){
  const f=fixture(),gate=deferred();let focusOptions;
  const input={value:'一半',isConnected:true,selectionStart:1,selectionEnd:2,selectionDirection:'backward',
   focus(options){focusOptions=options;f.context.document.activeElement=this},
   setSelectionRange(start,end,direction){this.selectionStart=start;this.selectionEnd=end;this.selectionDirection=direction},
  };
  const secret={get value(){throw Error('不应读取或复制密码值')}};
  const originalForm={id:'todo-form',input,secret};let currentForm=originalForm;
  f.context.document.activeElement=input;
  f.context.document.querySelectorAll=selector=>selector==='#main form[id]'?[currentForm]:f.controls;
  f.context.document.getElementById=()=>currentForm;
  f.context.render=()=>{
   input.isConnected=false;f.context.document.activeElement=null;
   currentForm={replaceWith(node){currentForm=node;input.isConnected=true}};
  };
  const save=f.context.api;
  f.context.api=async(path,body)=>{await gate.promise;if(conflict){if(body)throw Object.assign(Error('conflict'),{code:409});return {revision:5,data:createState()}}return save(path,body)};
  const pending=f.command('feed');
  input.value='等待时又写了几字';input.selectionStart=3;input.selectionEnd=5;
  gate.resolve();await pending;
  assert.equal(currentForm,originalForm);assert.equal(currentForm.input,input);assert.equal(currentForm.secret,secret);
  assert.equal(input.value,'等待时又写了几字');assert.equal(f.context.document.activeElement,input);
  assert.deepEqual([input.selectionStart,input.selectionEnd,input.selectionDirection],[3,5,'backward']);
  assert.equal(focusOptions.preventScroll,true);assert.equal(f.results[0].ok,!conflict);
  assert.ok(f.writes.every(body=>!JSON.stringify(body).includes('等待时又写了几字')));
 }
});
await check('shell size updates change existing controls without saving back',()=>{
 const field={value:'1'},label={textContent:'100%'};let received,saves=0;
 const context=vm.createContext({
  $:selector=>({'#pet-scale':field,'#pet-scale-value':label})[selector],
  szuDesktop:{onPetScale:callback=>{received=callback},setPetScale:()=>{saves++}},
 });
 vm.runInContext(source.slice(source.indexOf('function showPetScale('),source.indexOf('function render(){')),context);
 const registration=source.match(/globalThis\.szuDesktop\?\.onPetScale\?\.\(showPetScale\);/);
 assert.ok(registration);vm.runInContext(registration[0],context);
 received(1.35);assert.equal(field.value,'1.35');assert.equal(label.textContent,'135%');assert.equal(saves,0);
 context.$=()=>null;received(0.4);assert.equal(saves,0);
});
await check('preload strips IPC events, filters command names, and restricts result payloads',()=>{
 let bridge,listener,removed;const sent=[];
 const context=vm.createContext({require:()=>({
  contextBridge:{exposeInMainWorld:(_key,value)=>{bridge=value}},
  ipcRenderer:{on:(channel,fn)=>{assert.ok(['szu:pet-command','szu:pet-scale'].includes(channel));listener=fn},removeListener:(...args)=>{removed=args},send:(...args)=>sent.push(args)},
 })});
 vm.runInContext(readFileSync(new URL('./electron/preload.cjs',import.meta.url),'utf8'),context);
 const calls=[],unsubscribe=bridge.onPetCommand((...args)=>calls.push(args));
 for(const command of ['pat','feed','play','sleep','garden','farm','study','home','switchPet:0','switchPet:7','switchPet:8','switchPet:-1','switchPet:0.5','switchPet:__proto__','quit',{}])listener({sender:'private'},command);
 assert.equal(calls.length,10);assert.ok(calls.every(args=>args.length===1));
 unsubscribe();assert.equal(removed[0],'szu:pet-command');assert.equal(removed[1],listener);
 bridge.petResult({ok:true,message:'好'.repeat(121),secret:'never forward'});
 bridge.petResult({ok:'true',message:'invalid'});bridge.petResult({ok:true,message:5});
 assert.equal(sent.length,1);assert.equal(sent[0][0],'szu:pet-result');assert.equal(sent[0][1].message.length,120);
 assert.deepEqual(Object.keys(sent[0][1]),['ok','message']);assert.equal(Object.isFrozen(bridge),true);
 const scales=[],removeScale=bridge.onPetScale((...args)=>scales.push(args));
 for(const scale of [0.4,1.35,2,'1',Infinity,NaN,0.3,3])listener({sender:'private'},scale);
 assert.deepEqual(scales,[[0.4],[1.35],[2]]);removeScale();assert.equal(removed[0],'szu:pet-scale');assert.equal(removed[1],listener);
});
console.log(`${checks} pet command checks passed`);

import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const app=readFileSync(new URL('./assets/garden/app.mjs',import.meta.url),'utf8');
const tests=[];
function test(name,fn){tests.push([name,fn])}

// 设置页必须真的给出开机自启的入口和状态位置，否则后端接通了用户也看不到。
test('settings page exposes the autostart card',()=>{
  const at=app.indexOf('function settings(){');
  const settings=app.slice(at,app.indexOf('\n',at));
  assert.match(settings,/id="autostart-state"/);
  assert.match(settings,/btn\('读取中…','autostart'\)/);
  assert.match(settings,/只支持 Windows/);
  assert.match(app,/autostart:'i-signal'/,'开机自启按钮没有图标');
});

const paint=app.slice(app.indexOf('function paintAutostart(){'),app.indexOf('\n',app.indexOf('function paintAutostart(){')));

function paintWith(state){
  const nodes={'#autostart-state':{textContent:''}};
  const button={disabled:false,innerHTML:''};
  const ctx=vm.createContext({$:s=>nodes[s],document:{querySelector:()=>button},sprite:()=>'',autostartState:state});
  vm.runInContext(paint,ctx);
  vm.runInContext('paintAutostart()',ctx);
  return {text:nodes['#autostart-state'].textContent,button};
}

// 读不到状态时不能画成「未开启」，否则用户以为开关没生效，会反复点。
test('unknown autostart state is never painted as disabled',()=>{
  const unknown=paintWith({supported:true,detail:'状态未知：打不开注册表启动项',error:'打不开注册表启动项'});
  assert.match(unknown.text,/状态未知/);
  assert.doesNotMatch(unknown.text,/未开启/);
  const off=paintWith({supported:true,enabled:false,detail:'未开启'});
  assert.equal(off.text,'未开启');
  assert.match(off.button.innerHTML,/打开开机自启/);
  const on=paintWith({supported:true,enabled:true,detail:'已开启'});
  assert.match(on.button.innerHTML,/关掉开机自启/);
});

test('unsupported platform disables the button instead of pretending',()=>{
  const st=paintWith({supported:false,detail:'未开启（这个平台还没做）'});
  assert.equal(st.button.disabled,true);
  assert.match(st.text,/这个平台还没做/);
});

// 切换开关走的是真实点击分支：必须按当前状态取反、失败时不得报成功。
const click=app.slice(app.indexOf("document.addEventListener('click'"),app.indexOf("document.addEventListener('submit'"));

async function clickAutostart(initial,response){
  const calls=[];let message='',painted=0;
  const ctx=vm.createContext({
    probing:false,autostartState:initial,
    schoolUI:{click:async()=>false},campusUI:{click:async()=>false},
    document:{addEventListener:(_,handler)=>{ctx.clickHandler=handler}},
    run:fn=>{ctx.work=fn()},toast:t=>{message=t},paintAutostart:()=>{painted++},
    api:async(path,data)=>{calls.push([path,data]);if(response instanceof Error)throw response;return response},
  });
  vm.runInContext(click,ctx);
  ctx.clickHandler({target:{closest:()=>({dataset:{action:'autostart'}})},preventDefault:()=>{}});
  await ctx.work;
  return {calls,message,painted,state:ctx.autostartState};
}

// 请求对象来自 VM 里的另一个 realm，用 deepEqual 会比 prototype 而失败，逐项比原值。
function assertCall(calls,path,enabled){
  assert.equal(calls.length,1,'应该只发一次请求');
  assert.equal(calls[0][0],path);
  assert.equal(calls[0][1].enabled,enabled);
}

test('toggle posts the opposite of the current state',async()=>{
  const on=await clickAutostart({supported:true,enabled:false,detail:'未开启'},{supported:true,enabled:true,detail:'已开启 → szudesktop.exe'});
  assertCall(on.calls,'/api/autostart',true);
  assert.match(on.message,/已开启/);
  assert.equal(on.state.enabled,true);
  const off=await clickAutostart({supported:true,enabled:true,detail:'已开启'},{supported:true,enabled:false,detail:'未开启'});
  assertCall(off.calls,'/api/autostart',false);
  assert.equal(off.state.enabled,false);
});

test('a failed toggle reports the reason and keeps the old state',async()=>{
  const res=await clickAutostart({supported:true,enabled:false,detail:'未开启'},new Error('注册表被组策略锁定'));
  assert.match(res.message,/注册表被组策略锁定/);
  assert.doesNotMatch(res.message,/已打开|已关掉/);
  assert.equal(res.state.enabled,false);
  assert.equal(res.painted,1,'失败后也要重画，否则按钮文字和真实状态不一致');
});

for(const [name,fn] of tests){await fn();console.log('PASS',name)}
console.log(`${tests.length} autostart UI checks passed`);

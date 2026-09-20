import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import {networkBadge,networkSummary} from './assets/garden/network-status.mjs';

const connected={internet_ok:true,zone_label:'当前已联网',online_known:true,online:true};
assert.match(networkSummary(connected),/出口已在线/);
assert.match(networkSummary({...connected,online:false}),/未检测到在线会话/);
assert.match(networkSummary({...connected,online_known:false,online_error:'查询失败'}),/暂未查明：查询失败/);
assert.doesNotMatch(networkSummary({...connected,online_known:false}),/未检测到在线会话/);
console.log('PASS network reachability, authentication and unknown status remain distinct');

const app=readFileSync(new URL('./assets/garden/app.mjs',import.meta.url),'utf8');
const refresh=app.slice(app.indexOf('async function refresh(){'),app.indexOf('\nfunction networkResult'));
const nodes={'#network-summary':{textContent:''},'#network-badge':{textContent:''}};
const ctx=vm.createContext({networkSummary,networkBadge,$:s=>nodes[s],api:async()=>connected});
vm.runInContext('let net=null,saved=false,probing=false;'+refresh,ctx);
assert.equal(await vm.runInContext('refresh()',ctx),true);
assert.match(nodes['#network-summary'].textContent,/出口已在线/);
ctx.api=async()=>{throw Error('服务已退出')};
assert.equal(await vm.runInContext('refresh()',ctx),false);
assert.equal(nodes['#network-summary'].textContent,'服务已退出');
assert.equal(nodes['#network-badge'].textContent,'状态未确认');
assert.equal(vm.runInContext('net',ctx),null);
console.log('PASS failed refresh clears previous online state and reports failure');

const authenticate=app.slice(app.indexOf('async function authenticate('),app.indexOf('\nasync function run('));
let refreshed=0;
const authCtx=vm.createContext({credentialInput:()=>({data:{username:'test',password:'test'},remember:false}),networkResult:()=>{},api:async()=>({ok:false,message:'当前出口已有会话'}),refresh:async()=>{refreshed++}});
vm.runInContext(authenticate,authCtx);
await vm.runInContext('authenticate()',authCtx);
assert.equal(refreshed,1);
console.log('PASS a login that did not verify credentials still refreshes outlet status');
console.log('3 network UI checks passed');

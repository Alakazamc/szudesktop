import assert from 'node:assert/strict';
import {createNoticesUI} from './assets/garden/notices.mjs';
globalThis.document={getElementById:()=>({innerHTML:''})};
const sources=[{id:'undergrad',name:'教务部',url:'https://jwb.szu.edu.cn/index/jwtz.htm',group:'university',readable:true},{id:'college-fe',name:'教育学部',url:'https://fe.szu.edu.cn/xbzx/tzgg.htm',group:'college',readable:true},{id:'college-law',name:'法学院',url:'https://law.szu.edu.cn/xwjc/xygg.htm',group:'college',readable:true},{id:'college-csse',name:'计算机与软件学院',url:'https://csse.szu.edu.cn/',group:'college',readable:false,note:'暂未接入，请在学院官网查看。'}];
const response=id=>({source:sources.find(s=>s.id===id).name,items:[{title:id+' 公告 <script>',date:'2026-09-20',url:sources.find(s=>s.id===id).url}],fetched_at:'2026-09-20T12:00:00Z',stale:false});
let mode='normal',calls=[],pending=new Map(),count=0;
const ui=createNoticesUI({api:async path=>{calls.push(path);if(path.endsWith('/notice-sources'))return {sources};const id=new URL('http://localhost'+path).searchParams.get('source');if(mode==='defer')return new Promise(resolve=>pending.set(id,resolve));if(mode==='fail')throw Error('学校网站暂时无法读取');return response(id)}});
const choose=id=>ui.change({target:{id:'feed-source',value:id}});
async function check(name,f){await f();count++;console.log('PASS',name)}
await check('catalog groups university and college sources with their actual official links',async()=>{await ui.load();assert.match(ui.card(),/全校通知/);assert.match(ui.card(),/学院与学部/);assert.match(ui.card(),/计算机与软件学院 · 官网查看/)});
await check('choosing a college automatically reads its feed and escapes titles',async()=>{await choose('college-fe');assert.match(calls.at(-1),/source=college-fe/);assert.match(ui.card(),/教育学部 · 读取于/);assert.match(ui.card(),/&lt;script&gt;/);assert.match(ui.card(),/https:\/\/fe.szu.edu.cn\/xbzx\/tzgg.htm/)});
await check('unsupported college clears old notices and presents its official page without a fake read action',async()=>{const before=calls.length;await choose('college-csse');assert.equal(calls.length,before);assert.match(ui.card(),/打开学院官网/);assert.doesNotMatch(ui.card(),/教育学部 · 读取于|data-action="notice-read"/)});
await check('late response from a previous college never overwrites the selected feed',async()=>{mode='defer';const first=choose('college-fe');const second=choose('college-law');pending.get('college-law')(response('college-law'));await second;pending.get('college-fe')(response('college-fe'));await first;assert.match(ui.card(),/法学院 · 读取于/);assert.doesNotMatch(ui.card(),/教育学部 · 读取于/);mode='normal'});
await check('failed college change cannot keep a previous college feed under the new heading',async()=>{mode='fail';await choose('college-fe');assert.match(ui.card(),/学校网站暂时无法读取/);assert.doesNotMatch(ui.card(),/法学院 · 读取于/)});
console.log(`${count} notice checks passed`);

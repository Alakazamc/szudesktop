import assert from 'node:assert/strict';
import {createBookingUI,bookingSlotsHTML,createVenueRulesUI} from './assets/garden/booking.mjs';
const panel={innerHTML:''},rulesPanel={innerHTML:''};
globalThis.document={getElementById:id=>id==='booking-panel'?panel:id==='venue-rules-panel'?rulesPanel:null};
const day={room:{id:1,name:'测试会议室',description:'<img src=x>',type:{samePersonMaxReservationPerDay:4}},date:'2026-09-21',fetched_at:'2026-09-20T08:00:00Z',slots:[{index:28,start:'14:00',end:'14:30',state:'available'},{index:29,start:'14:30',end:'15:00',state:'occupied'}]};
// 学校 venue-api 的真实形态：type 内嵌在每条场地里，含类型名、开放时段掩码、
// 可提前天数、单日上限、爽约黑名单天数，以及一份全局统一的使用须知（纯文本，
// 因为 Go 层已经剥过标签）。掩码 4397794590720 数出来 20 个半小时格 = 10 小时。
const pianoMask=4397794590720,wideMask=268435455;
const notice='使用须知\n每人每日可预约4个时段，每时段半小时。\n<尖括号测试>';
const rooms=[
 {id:1,name:'时光社区共享琴房1（有钢琴）',campus:'粤海',community:'时光',status:true,description:'场地含钢琴1台',type:{id:1,name:'共享琴房',availableTimePeriod:pianoMask,samePersonMaxReservationPerDay:4,lastReservationDayBeforeAppointment:3,blacklistValidDuration:1,announcement:notice}},
 {id:2,name:'文山湖社区面试间1',campus:'粤海',community:'文山湖',status:true,description:'网络面试间',type:{id:2,name:'面试间',availableTimePeriod:wideMask,samePersonMaxReservationPerDay:4,lastReservationDayBeforeAppointment:3,blacklistValidDuration:1,announcement:notice}},
 {id:3,name:'沧海社区会议室1',campus:'丽湖',community:'沧海',status:true,description:'会议室',type:{id:3,name:'会议室',availableTimePeriod:wideMask,samePersonMaxReservationPerDay:4,lastReservationDayBeforeAppointment:3,blacklistValidDuration:1,announcement:notice}},
 {id:4,name:'沧海社区洽谈小组1',campus:'丽湖',community:'沧海',status:true,description:'洽谈小组',type:{id:4,name:'洽谈小组',availableTimePeriod:wideMask,samePersonMaxReservationPerDay:4,lastReservationDayBeforeAppointment:3,blacklistValidDuration:1,announcement:notice}}];
let calls=[],failure=false,empty=false,count=0;
const api=async path=>{calls.push(path);if(failure)throw Error('无法读取学校场地信息');if(path.endsWith('/rooms'))return {rooms:empty?[]:rooms,today:'2026-09-20'};if(path.includes('/availability'))return day;assert.fail('unexpected private API: '+path)};
const ui=createBookingUI({api});
const rules=createVenueRulesUI({api});
async function check(name,f){await f();count++;console.log('PASS',name)}
await check('booking goes directly to school, without a manual session or unfinished local form',()=>{
 assert.match(ui.card(),/href="https:\/\/swzx\.webvpn\.szu\.edu\.cn\/#\/pages\/booth\/szu-booth-list"/);
 assert.match(ui.card(),/登录并预约/);assert.match(ui.card(),/在浏览器打开学校页面/);
 assert.doesNotMatch(ui.card(),/Cookie|F12|booking-cookie|booking-form|booking-connect/);
 assert.equal(calls.length,0);
});
await check('availability is an escaped read-only overview, not a false selection step',()=>{
 const html=bookingSlotsHTML(day);assert.match(html,/空闲/);assert.match(html,/已预约/);assert.match(html,/&lt;img src=x&gt;/);
 assert.doesNotMatch(html,/<button|aria-pressed|已选中/);assert.match(html,/去学校页面预约/);
});
await check('query uses public endpoints and changing conditions discards previous slots',async()=>{
 await ui.click('booking-rooms');await ui.click('booking-query');assert.match(ui.card(),/14:00/);
 ui.change({target:{id:'booking-date',value:'2026-09-22'}});assert.doesNotMatch(ui.card(),/14:00/);assert.match(ui.card(),/条件已更改/);
 await ui.click('booking-query');assert.match(calls.at(-1),/date=2026-09-22/);
 assert.ok(calls.every(x=>x==='/api/booking/rooms'||x.startsWith('/api/booking/availability?')));
});
await check('query failure clears stale availability and keeps official booking reachable',async()=>{
 failure=true;await ui.click('booking-query');assert.doesNotMatch(ui.card(),/14:00/);assert.match(ui.card(),/无法读取学校场地信息/);assert.match(ui.card(),/登录并预约/);failure=false;
});
await check('empty room list is explicit and still offers the school page',async()=>{
 empty=true;await ui.click('booking-rooms');assert.match(ui.card(),/没有返回可查询的场地/);assert.doesNotMatch(ui.card(),/id="booking-room"/);assert.match(ui.card(),/登录并预约/);empty=false;
});
await check('retired private actions cannot send a session or reservation request',async()=>{
 const before=calls.length;
 for(const action of ['booking-connect','booking-history','booking-slot','booking-commit'])assert.equal(await ui.click(action),false);
 assert.equal(calls.length,before);
});
// —— 场地与琴房规则速查（用学校已经返回、此前被丢掉的字段）——
await check('rules card groups rooms by venue type from the same public endpoint',async()=>{
 await rules.click('booking-rules');
 const html=rules.card();
 assert.match(html,/场地与琴房规则速查/);
 for(const t of ['共享琴房','面试间','会议室','洽谈小组'])assert.match(html,new RegExp(t));
 assert.match(html,/4 个场地/);
 assert.ok(calls.every(x=>x==='/api/booking/rooms'||x.startsWith('/api/booking/availability?')),'只能用公开只读端点');
});
await check('school announcement is plain text and shown once, not once per venue',()=>{
 const html=rules.card();
 // 标签只出现一次；正文里的特征句也只出现一次（按 4 个场地重复就会出现 4 次）
 assert.equal((html.match(/学校统一使用须知/g)||[]).length,1,'统一须知的标题只出现一次');
 assert.equal((html.match(/每人每日可预约4个时段/g)||[]).length,1,'须知正文不得按场地重复');
 // 只针对学校内容那一块：必须是转义后的纯文本，一个尖括号都不该有
 const body=/<pre class="notice">([\s\S]*?)<\/pre>/.exec(html);
 assert.ok(body,'须知应以纯文本块呈现');
 assert.doesNotMatch(body[1],/[<>]/,'学校内容不得以标签形态进入页面，必须转义成文本');
 assert.doesNotMatch(body[1],/onerror|script/,'脚本与事件属性必须被剥掉');
 assert.match(body[1],/&lt;尖括号测试&gt;/,'文本里的尖括号必须被转义');
});
await check('per-type rules show the school\'s own numbers, not our guesses',()=>{
 const html=rules.card();
 assert.match(html,/单日 4 格/);assert.match(html,/可提前 3 天/);assert.match(html,/爽约 1 天/);
 assert.match(html,/每日开放 10 小时/);assert.match(html,/每日开放 14 小时/);
});
await check('rules card carries no photos and still routes booking to the school page',()=>{
 const html=rules.card();
 assert.doesNotMatch(html,/<img/,'学校图片路径拼不出来，这轮不热链');
 assert.match(html,/学校页面/);
});
await check('rules failure is explicit and keeps the official page reachable',async()=>{
 failure=true;await rules.click('booking-rules');
 assert.match(rules.card(),/无法读取学校场地信息/);assert.match(rules.card(),/登录并预约/);failure=false;
});
console.log(`${count} booking checks passed`);

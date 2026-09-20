import assert from 'node:assert/strict';
import {createBookingUI,bookingSlotsHTML} from './assets/garden/booking.mjs';
const panel={innerHTML:''};
globalThis.document={getElementById:id=>id==='booking-panel'?panel:null};
const day={room:{id:1,name:'测试会议室',description:'<img src=x>',type:{samePersonMaxReservationPerDay:4}},date:'2026-09-21',fetched_at:'2026-09-20T08:00:00Z',slots:[{index:28,start:'14:00',end:'14:30',state:'available'},{index:29,start:'14:30',end:'15:00',state:'occupied'}]};
let calls=[],failure=false,empty=false,count=0;
const ui=createBookingUI({api:async path=>{calls.push(path);if(failure)throw Error('无法读取学校场地信息');if(path.endsWith('/rooms'))return {rooms:empty?[]:[{...day.room,status:true}],today:'2026-09-20'};if(path.includes('/availability'))return day;assert.fail('unexpected private API: '+path)}});
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
 empty=true;await ui.click('booking-rooms');assert.match(ui.card(),/没有返回可查询的场地/);assert.doesNotMatch(ui.card(),/id="booking-room"/);assert.match(ui.card(),/登录并预约/);
});
await check('retired private actions cannot send a session or reservation request',async()=>{
 const before=calls.length;
 for(const action of ['booking-connect','booking-history','booking-slot','booking-commit'])assert.equal(await ui.click(action),false);
 assert.equal(calls.length,before);
});
console.log(`${count} booking checks passed`);

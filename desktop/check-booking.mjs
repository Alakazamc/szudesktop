import assert from 'node:assert/strict';
import {createBookingUI,bookingSlotsHTML} from './assets/garden/booking.mjs';
const nodes=new Map();const node=id=>{if(!nodes.has(id))nodes.set(id,{innerHTML:'',value:''});return nodes.get(id)};
globalThis.document={getElementById:node};
const day={room:{id:1,name:'测试会议室',description:'<img src=x>',type:{samePersonMaxReservationPerDay:4}},date:'2026-09-21',fetched_at:'2026-09-20T08:00:00Z',slots:[{index:28,start:'14:00',end:'14:30',state:'available'},{index:29,start:'14:30',end:'15:00',state:'occupied'}]};
let approve=false,expired=false,calls=[],count=0;
const ui=createBookingUI({toast(){},confirm:async()=>approve,api:async(path,body,method)=>{calls.push({path,body,method});if(path.endsWith('/session'))return {authenticated:method!=='DELETE'};if(expired)throw Object.assign(Error('登录失效'),{code:401});if(path.endsWith('/rooms'))return {rooms:[{...day.room,status:true}],today:'2026-09-20'};if(path.includes('/availability'))return day;if(path.endsWith('/prepare'))return {token:'once',room:'测试会议室',date:day.date,times:['14:00–14:30']};if(path.endsWith('/commit'))return {ok:true,message:'已提交'};return {records:[],total:0,page:1}}});
async function check(name,f){await f();count++;console.log('PASS',name)}
await check('slots have labels, selection state, disabled occupancy and escaped content',()=>{
 const html=bookingSlotsHTML(day,[28]);assert.match(html,/已选中/);assert.match(html,/已预约/);assert.match(html,/&lt;img src=x&gt;/);assert.match(html,/data-index="29"[^>]+disabled/);assert.match(html,/aria-pressed="true"/);
});
await check('live conditions invalidate old slots and selection',async()=>{
 await ui.click('booking-rooms');await ui.click('booking-query');assert.match(ui.card(),/14:00/);
 ui.change({target:{id:'booking-date',value:'2026-09-21'}});assert.doesNotMatch(ui.card(),/data-index="28"/);assert.match(ui.card(),/条件已更改/);
});
await check('cancelled review cannot submit a reservation',async()=>{
 await ui.load();await ui.click('booking-query');await ui.click('booking-slot',{dataset:{index:'28'}});
 await ui.submit({id:'booking-form'},{phone:'13800000000',grade:'2025',agree:'on'});
 assert.equal(calls.filter(x=>x.path.endsWith('/commit')).length,0);
});
await check('explicit confirmation uses a one-time token, never repeats personal payload',async()=>{
 approve=true;await ui.submit({id:'booking-form'},{phone:'13800000000',grade:'2025',agree:'on'});
 const commits=calls.filter(x=>x.path.endsWith('/commit'));assert.equal(commits.length,1);assert.deepEqual(commits[0].body,{token:'once'});assert.match(ui.card(),/学校本次返回空预约列表/);
});
await check('expired login removes cached history and submission form',async()=>{
 expired=true;await ui.click('booking-history');assert.doesNotMatch(ui.card(),/id="booking-form"/);assert.doesNotMatch(ui.card(),/学校本次返回空预约列表/);assert.match(ui.card(),/登录失效/);
});
console.log(`${count} booking checks passed`);

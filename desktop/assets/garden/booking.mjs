import {pixelIcon} from './pixel.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const official='https://swzx.webvpn.szu.edu.cn/#/pages/booth/szu-booth-list';
const labels={available:'空闲',occupied:'已预约',closed:'不可选',past:'已开始',unknown:'未确认'};
const tones={available:'success',occupied:'muted',closed:'muted',past:'muted',unknown:'warning'};
const button=(text,key)=>`<button type="button" data-action="booking-${key}">${text}</button>`;
const officialLink=(text,cls='button primary')=>`<a class="${cls}" href="${official}" target="_blank" rel="noopener noreferrer">${pixelIcon('i-key')}${text} ↗</a>`;
export function bookingSlotsHTML(day){
 if(!day)return '<p class="empty">选择场地和日期，点击「查询空位」。</p>';
 return `<p>${esc(day.room.description)}</p><p class="muted">每格 30 分钟 · 单日最多 ${day.room.type.samePersonMaxReservationPerDay} 格，已预约的时段也计入上限。空闲不代表当前账号有预约权限。</p><ul class="booking-slots">${day.slots.map(s=>`<li class="booking-slot" data-tone="${tones[s.state]||'warning'}"><b>${esc(s.start)}–${esc(s.end)}</b><small>${esc(labels[s.state]||'未确认')}</small></li>`).join('')}</ul>${day.slots.length?'':'<p class="empty">这个场地当天没有开放时段。</p>'}<p class="muted">读取于 ${esc(new Date(day.fetched_at).toLocaleTimeString('zh-CN'))}；空位可能变化，请在学校页面选择时段并确认预约。</p><div class="actions">${officialLink('去学校页面预约')}</div>`;
}
// 开放时段掩码是 64 位整数，超过 32 位，JS 的位运算会截断，只能用 BigInt 数位数。
const openHours=m=>{let x=BigInt(m||0),n=0;for(let i=0n;i<64n;i++)if((x>>i)&1n)n++;return n/2};
export function venueRulesHTML(rooms){
 if(!rooms.length)return '<p class="empty">还没有读取学校场地规则。</p>';
 const groups=new Map();
 for(const r of rooms){const t=r.type||{};const name=t.name||'未分类';const g=groups.get(name)||{name,type:t,rooms:[]};g.rooms.push(r);groups.set(name,g)}
 const list=[...groups.values()];
 // 四类场地的使用须知是同一份文本（学校返回的 sha 相同），所以只显示一次；
 // 按场地重复会造成「每个场地规则不同」的错觉。
 const notice=(rooms.find(r=>r.type&&r.type.announcement)||{}).type?.announcement||'';
 return `<p class="muted">学校返回 ${esc(rooms.length)} 个场地、${esc(list.length)} 类。规则与设备说明取自学校场地接口的原文，办理预约仍在学校页面完成。</p>
 ${notice?`<details open><summary>学校统一使用须知</summary><pre class="notice">${esc(notice)}</pre></details>`:'<p class="muted">学校这次没有返回使用须知。</p>'}
 ${list.map(g=>`<section class="venue-group"><h3>${esc(g.name)} · ${esc(g.rooms.length)} 处</h3><p class="muted">单日 ${esc(g.type.samePersonMaxReservationPerDay??'—')} 格 · 可提前 ${esc(g.type.lastReservationDayBeforeAppointment??'—')} 天 · 每日开放 ${esc(openHours(g.type.availableTimePeriod))} 小时 · 爽约 ${esc(g.type.blacklistValidDuration??'—')} 天内不可再约</p><ul class="venue-rules">${g.rooms.map(r=>`<li><strong>${esc(r.name)}</strong><small>${esc(r.campus||'')}${r.community?' · '+esc(r.community):''}${r.status?'':' · 已停用'}</small><p class="muted">${esc(r.description||'学校未提供设备说明')}</p></li>`).join('')}</ul></section>`).join('')}
 <p class="muted">图片与现场实况以学校页面为准，本页不内嵌学校图片。</p><div class="actions">${officialLink('登录并预约')}</div>`;
}
export function createVenueRulesUI({api}){
 let rooms=[],error='',busy=false;
 function content(){
  return `<div class="actions">${button(pixelIcon('i-compass')+'读取学校场地规则','rules')}</div>
 ${error?`<p role="status" class="notice error">${esc(error)}</p><div class="actions">${officialLink('登录并预约')}</div>`:''}
 ${rooms.length?venueRulesHTML(rooms):'<p class="muted">规则来自学校场地接口，不需要登录。读取后可看每类场地的时段上限、可提前天数与爽约限制。</p>'}`;
 }
 function card(){return `<section class="card campus-booking" id="venue-rules-panel"><div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-book','heading-icon')}场地与琴房规则速查</h2><span class="badge" data-tone="info">只读 · 学校返回原文</span></div>${content()}</section>`}
 function paint(){const el=document.getElementById('venue-rules-panel');if(el)el.innerHTML=content()}
 async function click(a){
  if(a!=='booking-rules')return false;
  if(busy)return true;
  busy=true;error='';paint();
  try{const data=await api('/api/booking/rooms');rooms=data.rooms||[];paint()}
  catch(e){rooms=[];error=e.message;paint()}
  finally{busy=false;paint()}
  return true;
 }
 return {card,click,paint};
}
export function createBookingUI({api}){
 let rooms=[],today='',room='',date='',day=null,error='',message='',busy=false;
 function content(){return `<div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-calendar','heading-icon')}学习空间 · 预约与空位</h2><span class="badge" data-tone="info">学校页面办理</span></div>
 <p>社区会议室、面试间与琴房。登录、选择时段、提交和查看预约结果，都在学校官方页面完成。</p>
 <div class="actions">${officialLink('登录并预约')}${button(pixelIcon('i-compass')+'查看场地空位','rooms')}</div>
 <p class="muted">点击「登录并预约」会在浏览器打开学校页面，按学校提示完成登录即可。图书馆使用独立预约系统。</p>
 ${rooms.length?`<h3>空位速览</h3><p>这里可以先查空位；具体预约资格与最终结果以学校系统为准。</p><div class="grid"><div><label for="booking-room">场地 · ${rooms.length} 处</label><select id="booking-room">${rooms.map(x=>`<option value="${x.id}" ${String(x.id)===room?'selected':''}>${esc(x.campus)} · ${esc(x.name)}${x.status?'':'（停用）'}</option>`).join('')}</select></div><div><label for="booking-date">使用日期</label><input id="booking-date" type="date" value="${esc(date)}" min="${esc(today)}"></div></div><div class="actions">${button('查询空位','query')}</div>`:''}
 <div role="status" aria-live="polite" class="notice" data-tone="${error?'error':'info'}">${esc(error||message||'校园网内可在这里直接查看场地空位，无需先登录。')}</div>
 ${rooms.length?bookingSlotsHTML(day):''}`}
 function card(){return `<section class="card campus-booking" id="booking-panel">${content()}</section>`}
 function paint(){const el=document.getElementById('booking-panel');if(el)el.innerHTML=content()}
 async function click(a){
  if(!['booking-rooms','booking-query'].includes(a))return false;
  if(busy)return true;
  busy=true;error='';message='正在读取学校场地信息…';paint();
  try{
   if(a==='booking-rooms'){
    rooms=[];day=null;
    const data=await api('/api/booking/rooms');rooms=data.rooms;today=data.today;date=today;room=String(rooms.find(x=>x.status)?.id||rooms[0]?.id||'');
    message=rooms.length?`已从学校读取 ${rooms.length} 个场地`:'学校本次没有返回可查询的场地，请打开官方页面查看。';
   }else{
    day=null;
    day=await api(`/api/booking/availability?room=${encodeURIComponent(room)}&date=${encodeURIComponent(date)}`);
    message='已读取学校空位。预约请点击「去学校页面预约」。';
   }
  }catch(e){error=e.message}
  finally{busy=false;paint()}
  return true;
 }
 function change(e){
  if(!['booking-room','booking-date'].includes(e.target.id))return false;
  if(e.target.id==='booking-room')room=e.target.value;else date=e.target.value;
  day=null;error='';message='条件已更改，请重新查询空位';paint();return true;
 }
 return {card,click,change};
}

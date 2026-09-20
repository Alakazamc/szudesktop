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

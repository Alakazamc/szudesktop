import {pixelIcon} from './pixel.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const official='https://swzx.webvpn.szu.edu.cn/#/pages/booth/szu-booth-list';
const labels={available:'空闲',occupied:'已预约',closed:'不可选',past:'已开始',unknown:'未确认'};
const tones={available:'success',occupied:'muted',closed:'muted',past:'muted',unknown:'warning'};
const button=(text,key,extra='')=>`<button data-action="booking-${key}" ${extra}>${text}</button>`;
export function bookingSlotsHTML(day,selected=[]){
 if(!day)return '<p class="empty">选择场地和日期，点击「查询空位」。</p>';
 return `<p>${esc(day.room.description)}</p><p class="muted">每格 30 分钟 · 单日最多 ${day.room.type.samePersonMaxReservationPerDay} 格，已预约的时段也计入上限。空闲不代表当前账号有预约权限。</p><div class="booking-slots">${day.slots.map(s=>button(`<b>${esc(s.start)}–${esc(s.end)}</b><small>${selected.includes(s.index)?'已选中':esc(labels[s.state]||'未确认')}</small>`,'slot',`data-index="${s.index}" data-tone="${selected.includes(s.index)?'info':tones[s.state]||'warning'}" aria-pressed="${selected.includes(s.index)}" ${s.state==='available'?'':'disabled'}`)).join('')}</div>${day.slots.length?'':'<p class="empty">这个场地当天没有开放时段。</p>'}<p class="muted">读取于 ${esc(new Date(day.fetched_at).toLocaleTimeString('zh-CN'))}；已开始的时段在本应用中不可选择。提交前会重新检查空位。</p>`;
}
export function createBookingUI({api,toast,confirm}){
 let rooms=[],today='',room='',date='',day=null,selected=[],authenticated=false,error='',message='',busy=false,history=null;
 function content(){return `<div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-calendar','heading-icon')}学习空间 · 实时空位</h2><span class="badge" data-tone="warning">预约接入测试</span></div><p>社区会议室、面试间与琴房。空位从学校实时读取；请按场地用途使用。图书馆座位使用独立系统。</p><div class="actions">${button(pixelIcon('i-compass')+'读取场地','rooms')}<a class="button quiet" href="${official}" target="_blank" rel="noopener noreferrer">官方预约页面 ↗</a></div>${rooms.length?`<div class="grid"><div><label for="booking-room">场地 · ${rooms.length} 处</label><select id="booking-room">${rooms.map(x=>`<option value="${x.id}" ${String(x.id)===room?'selected':''}>${esc(x.campus)} · ${esc(x.name)}${x.status?'':'（停用）'}</option>`).join('')}</select></div><div><label for="booking-date">使用日期</label><input id="booking-date" type="date" value="${esc(date)}" min="${esc(today)}"></div></div><div class="actions">${button('查询空位','query')}</div>`:''}<div role="status" aria-live="polite" class="notice" data-tone="${error?'error':'info'}">${esc(error||message||'校园网内可直接查公开空位；提交预约需另行验证预约登录状态。')}</div>${bookingSlotsHTML(day,selected)}<details><summary>预约登录 · ${authenticated?'已验证（本次运行）':'尚未连接'}</summary><p>先在上方学校官方预约页面完成 WebVPN 登录，再进入场地详情。按 F12 → 网络，选中发往 <code>swzx.webvpn.szu.edu.cn</code>、路径包含 <code>venue-api</code> 的请求，复制请求标头中的 Cookie。</p><p class="muted">仅粘到下面。预约 Cookie 与成绩 Cookie 分开，仅保留在本次运行内存中，通过 HTTPS 发给学校 WebVPN；不要发到聊天。</p><label for="booking-cookie">预约页面的 Cookie</label><input id="booking-cookie" type="password" autocomplete="off" maxlength="8000" placeholder="粘贴预约专用 Cookie"><div class="actions">${button('验证预约登录','connect')}${button('清除本次登录','disconnect',authenticated?'':'disabled')}</div></details>${authenticated?`<form id="booking-form" autocomplete="off"><h3>核对并提交预约</h3><p>本地提交尚待真实预约验收。先选择上方空闲时段，再填写学校要求的信息。</p><div class="grid"><div><label for="booking-phone">联系电话</label><input id="booking-phone" name="phone" type="tel" pattern="1[0-9]{10}" maxlength="11" required></div><div><label for="booking-grade">入学年份</label><input id="booking-grade" name="grade" inputmode="numeric" pattern="[0-9]{4}" maxlength="4" required placeholder="例如 2025"></div></div><details><summary>场地使用须知</summary><p>每人每日最多预约 4 个半小时时段，不可同时预约多个场地；迟到 15 分钟记爽约，一个月内三次爽约会暂停预约权限。请勿用餐、带外卖或含糖饮料，使用后带走物品。</p><p class="muted">不同场地的最新规则与账号可预约范围以学校原页面为准。提交前请打开原页阅读所选场地完整须知。</p></details><label class="booking-consent"><input type="checkbox" name="agree" required>我已阅读官方完整须知，同意遵守，并确认场地用途合适</label><button class="primary" type="submit" ${selected.length?'':'disabled'}>${pixelIcon('i-quill')}核对所选 ${selected.length} 个时段</button></form>`:''}<div class="actions">${button('刷新我的预约','history',authenticated?'':'disabled')}</div>${historyHTML()}`}
 function historyHTML(){if(!history)return '';return `<h3>我的预约 · 共 ${history.total} 条</h3>${history.records.length?`<ul class="booking-history">${history.records.map(x=>`<li><strong>${esc(x.booth.name)}</strong><p>${esc(x.date)} · ${esc(x.reservationStartTime)}–${esc(x.reservationEndTime)} · ${esc(['未使用','已取消','已使用','已过期'][x.status]||'状态待核对')}</p></li>`).join('')}</ul>`:'<p>学校本次返回空预约列表。</p>'}<div class="actions">${button('上一页','previous',history.page>1?'':'disabled')}<span>第 ${history.page} 页</span>${button('下一页','next',history.page*30<history.total?'':'disabled')}</div><small>不显示校园卡号、门锁密码或手机号。取消预约及到场开锁请在学校原页面操作。</small>`}
 function card(){return `<section class="card campus-booking" id="booking-panel">${content()}</section>`}
 function paint(){const el=document.getElementById('booking-panel');if(el)el.innerHTML=content()}
 async function load(){try{const s=await api('/api/booking/session');authenticated=!!s.authenticated}catch{authenticated=false}paint()}
 async function readDay(){selected=[];day=null;day=await api(`/api/booking/availability?room=${encodeURIComponent(room)}&date=${encodeURIComponent(date)}`)}
 async function readHistory(page=1){history=null;history=await api('/api/booking/history?page='+page)}
 async function click(a,b){
  if(!a.startsWith('booking-'))return false;
  if(busy)return true;
  if(a==='booking-slot'){const index=Number(b.dataset.index),slot=day?.slots.find(x=>x.index===index);if(slot?.state!=='available')return true;if(selected.includes(index))selected=selected.filter(x=>x!==index);else if(selected.length<day.room.type.samePersonMaxReservationPerDay)selected.push(index);else toast('已达到该场地单日时段上限');paint();return true}
  busy=true;error='';message='正在读取学校预约服务…';
  const cookie=a==='booking-connect'?document.getElementById('booking-cookie')?.value:'';
  if(a==='booking-connect'){const el=document.getElementById('booking-cookie');if(el)el.value='';authenticated=false;history=null}
  paint();
  try{
   if(a==='booking-rooms'){rooms=[];day=null;selected=[];const data=await api('/api/booking/rooms');rooms=data.rooms;today=data.today;date=today;room=String(rooms.find(x=>x.status)?.id||rooms[0]?.id||'');message=`已从学校读取 ${rooms.length} 个场地`}
   else if(a==='booking-query'){await readDay();message='已读取学校空位。查询结果可能随其他同学的预约变化。'}
   else if(a==='booking-connect'){await api('/api/booking/session',{cookie});authenticated=true;message='预约登录已通过本人预约记录查询验证；关闭应用即清除';toast('预约登录已连接')}
   else if(a==='booking-disconnect'){await api('/api/booking/session',undefined,'DELETE');authenticated=false;history=null;selected=[];message='已清除本次预约登录'}
   else if(['booking-history','booking-previous','booking-next'].includes(a)){const page=a==='booking-previous'?history.page-1:a==='booking-next'?history.page+1:1;await readHistory(page);message='已读取本人预约记录'}
  }catch(e){error=e.message;if(e.code===401){authenticated=false;history=null}}
  finally{busy=false;paint()}
  return true;
 }
 function change(e){if(!['booking-room','booking-date'].includes(e.target.id))return false;if(e.target.id==='booking-room')room=e.target.value;else date=e.target.value;selected=[];day=null;error='';message='条件已更改，请重新查询空位';paint();return true}
 async function submit(form,values){
  if(form.id!=='booking-form')return false;
  if(busy)return true;busy=true;error='';
  try{
   const p=await api('/api/booking/prepare',{boothId:Number(room),date,timeList:[...selected],phone:values.phone,grade:values.grade,agree:!!values.agree});
   if(!await confirm('确认向学校提交预约？',`${p.room}\n${p.date}\n${p.times.join('、')}\n提交后会占用这些时段，请按时到场。`))return true;
   const result=await api('/api/booking/commit',{token:p.token});message=result.message;selected=[];day=null;toast(result.message);await readHistory();
  }catch(e){error=e.message;if(e.code===401){authenticated=false;history=null}}
  finally{values.phone='';values.grade='';busy=false;paint()}
  return true;
 }
 return {card,load,click,change,submit};
}

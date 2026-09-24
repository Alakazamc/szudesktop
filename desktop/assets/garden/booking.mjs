import {pixelIcon} from './pixel.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const official='https://swzx.webvpn.szu.edu.cn/#/pages/booth/szu-booth-list';
const labels={available:'空闲',occupied:'已预约',closed:'不可选',past:'已开始',unknown:'未确认'};
const tones={available:'success',occupied:'muted',closed:'muted',past:'muted',unknown:'warning'};
const button=(text,key,disabled=false)=>`<button type="button" data-action="booking-${key}"${disabled?' disabled':''}>${text}</button>`;
const officialLink=(text,cls='button primary')=>`<a class="${cls}" href="${official}" target="_blank" rel="noopener noreferrer">${pixelIcon('i-key')}${text} ↗</a>`;
export function bookingSlotsHTML(day){
 if(!day)return '<p class="empty">选择场地和日期，点击「查询空位」。</p>';
 try{return bookingSlotsBody(day)}
 catch(e){return `<p class="notice error">空位这次没能显示（${esc(e&&e.message||'数据格式异常')}），请在官方页面查看。</p>`}
}
function bookingSlotsBody(day){
 if(!Array.isArray(day.slots))throw Error('空位数据格式异常');
 return `<p>${esc(day.room.description)}</p><p class="muted">每格 30 分钟 · 单日最多 ${esc(ruleNum(day.room.type.samePersonMaxReservationPerDay))} 格，已预约的时段也计入上限。空闲不代表当前账号有预约权限。</p><ul class="booking-slots">${day.slots.map(s=>`<li class="booking-slot" data-tone="${tones[s.state]||'warning'}"><b>${esc(s.start)}–${esc(s.end)}</b><small>${esc(labels[s.state]||'未确认')}</small></li>`).join('')}</ul>${day.slots.length?'':'<p class="empty">这个场地当天没有开放时段。</p>'}<p class="muted">读取于 ${esc(new Date(day.fetched_at).toLocaleTimeString('zh-CN'))}；空位可能变化，请在学校页面选择时段并确认预约。</p><div class="actions">${officialLink('去学校页面预约')}</div>`;
}
// 开放时段掩码是 64 位整数，超过 32 位，JS 的位运算会截断，只能用 BigInt 数位数。
//
// ⚠️ 这个函数在页面渲染的关键路径上：services() 里被无条件求值，而它的结果会被整段
// 赋给 #main。一旦抛异常，那句赋值就不会执行 —— 整个「校园服务」页静默不更新、
// 且不会向用户报任何错。所以任何不可信输入（非整数、带千分位、超出安全整数范围、
// 负数）一律降级成「—」，绝不抛。学校侧字段格式一改就让整页白板，代价太大。
const openHours=m=>{try{
 if(m===null||m===undefined||m==='')return '—';
 if(typeof m==='number'&&!Number.isSafeInteger(m))return '—';
 if(typeof m==='string'&&!/^\d+$/.test(m.trim()))return '—';
 const x=BigInt(typeof m==='string'?m.trim():m);
 if(x<=0n)return '—';
 let n=0;for(let i=0n;i<64n;i++)if((x>>i)&1n)n++;
 return n%2?(n/2).toFixed(1):String(n/2);
}catch{return '—'}};
// 规则数字统一走这里：学校给的必须是正整数，越界、缺失、小数、非数字一律显示「—」。
// 校验用 isSafeInteger 而不是 isFinite —— 它同时挡住小数、Infinity、NaN 和超出安全整数的值，
// 这些都不是「学校给的规则数字」，让它们以任何形式出现在页面上都是在编造信息。
// 0 也显示「—」：服务端会把越界值收敛成 0（见 bookingType.validate），我们分不清
// 「学校真的报了 0」和「是我们自己清零的」，宁可标成未知，也不拿 0 冒充学校数字。
const ruleNum=v=>Number.isSafeInteger(v)&&v>0?String(v):'—';
// 对外只暴露这个包装：内部无论遇到什么形状的数据，都只降级这一块，绝不向上抛。
export function venueRulesHTML(rooms){
 try{return venueRulesBody(Array.isArray(rooms)?rooms:[])}
 catch(e){return `<p class="empty">场地规则这次没能显示（${esc(e&&e.message||'数据格式异常')}）。可以稍后重试，或直接到学校页面查看。</p>`}
}
function venueRulesBody(rooms){
 if(!rooms.length)return '<p class="empty">还没有读取学校场地规则。</p>';
 const groups=new Map();
 // 按 typeId 分组，不按 name。name 是展示字符串，不同类型重名时按 name 分组会把两类场地
 // 并成一组，并顺手用第一个的规则去描述另一类 —— 规则张冠李戴比不显示更糟。
 for(const r of rooms){const t=r.type||{};const key=String(r.typeId||t.name||'未分类');const g=groups.get(key)||{name:t.name||'未分类',type:t,rooms:[]};g.rooms.push(r);groups.set(key,g)}
 const list=[...groups.values()];
 // 四类场地的使用须知是同一份文本（学校返回的 sha 相同），所以只显示一次；
 // 按场地重复会造成「每个场地规则不同」的错觉。
 const notice=(rooms.find(r=>r.type&&r.type.announcement)||{}).type?.announcement||'';
 return `<p class="muted">学校返回 ${esc(rooms.length)} 个场地、${esc(list.length)} 类。规则与设备说明取自学校场地接口的原文，办理预约仍在学校页面完成。</p>
 ${notice?`<details open><summary>学校统一使用须知</summary><pre class="notice">${esc(notice)}</pre></details>`:'<p class="muted">学校这次没有返回使用须知。</p>'}
 ${list.map(g=>`<section class="venue-group"><h3>${esc(g.name)} · ${esc(g.rooms.length)} 处</h3><p class="muted">单日 ${esc(ruleNum(g.type.samePersonMaxReservationPerDay))} 格 · 可提前 ${esc(ruleNum(g.type.lastReservationDayBeforeAppointment))} 天 · 每日开放 ${esc(openHours(g.type.availableTimePeriod))} 小时 · 爽约 ${esc(ruleNum(g.type.blacklistValidDuration))} 天内不可再约</p><ul class="venue-rules">${g.rooms.map(r=>`<li><strong>${esc(r.name)}</strong><small>${esc(r.campus||'')}${r.community?' · '+esc(r.community):''}${r.status?'':' · 已停用'}</small><p class="muted">${esc(r.description||'学校未提供设备说明')}</p></li>`).join('')}</ul></section>`).join('')}
 <p class="muted">图片与现场实况以学校页面为准，本页不内嵌学校图片。</p><div class="actions">${officialLink('登录并预约')}</div>`;
}
// 「学校场地列表」是两张卡片共用的同一次只读请求。这个加载器带短 TTL 并合并并发调用：
// 先后点两张卡的按钮只会打一次接口；失败不写入缓存，下次点击照旧重试。
export function createRoomsLoader(api,ttl=60000){
 let cache=null,cachedAt=0,pending=null;
 return ()=>{
  if(cache&&Date.now()-cachedAt<ttl)return Promise.resolve(cache);
  if(pending)return pending;
  pending=api('/api/booking/rooms').then(d=>{cache=d;cachedAt=Date.now();pending=null;return d},e=>{pending=null;throw e});
  return pending;
 };
}
export function createVenueRulesUI({api,loadRooms}){
 // loadRooms 由上层注入（两张卡片共享一次请求）；单独使用时退回直连。
 const fetchRooms=loadRooms||(()=>api('/api/booking/rooms'));
 let rooms=[],error='',busy=false;
 function content(){
  return `<div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-book','heading-icon')}场地与琴房规则速查</h2><span class="badge" data-tone="info">只读 · 学校返回原文</span></div><div class="actions">${button(pixelIcon('i-compass')+'读取学校场地规则','rules',busy)}</div>
 ${busy?'<p role="status">正在读取学校场地规则…</p>':''}
 ${error?`<p role="status" class="notice error">${esc(error)}</p><div class="actions">${officialLink('登录并预约')}</div>`:''}
 ${rooms.length?venueRulesHTML(rooms):'<p class="muted">规则来自学校场地接口，不需要登录。读取后可看每类场地的时段上限、可提前天数与爽约限制。</p>'}`;
 }
 function card(){return `<section class="card campus-booking" id="venue-rules-panel">${content()}</section>`}
 function paint(){const el=document.getElementById('venue-rules-panel');if(el)el.innerHTML=content()}
 async function click(a){
  if(a!=='booking-rules')return false;
  if(busy)return true;
  busy=true;error='';paint();
  try{const data=await fetchRooms();rooms=data.rooms||[];paint()}
  catch(e){rooms=[];error=e.message;paint()}
  finally{busy=false;paint()}
  return true;
 }
 return {card,click,paint};
}
export function createBookingUI({api,loadRooms}){
 // 同上：优先用上层共享的请求，单独使用时直连。
 const fetchRooms=loadRooms||(()=>api('/api/booking/rooms'));
 let rooms=[],today='',room='',date='',day=null,error='',message='',busy=false,requestVersion=0;
 function content(){return `<div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-calendar','heading-icon')}学习空间 · 预约与空位</h2><span class="badge" data-tone="info">学校页面办理</span></div>
 <p>社区会议室、面试间与琴房。登录、选择时段、提交和查看预约结果，都在学校官方页面完成。</p>
 <div class="actions">${officialLink('登录并预约')}${button(pixelIcon('i-compass')+'查看场地空位','rooms',busy)}</div>
 <p class="muted">点击「登录并预约」会在浏览器打开学校页面，按学校提示完成登录即可。图书馆使用独立预约系统。</p>
 ${rooms.length?`<h3>空位速览</h3><p>这里可以先查空位；具体预约资格与最终结果以学校系统为准。</p><div class="grid"><div><label for="booking-room">场地 · ${rooms.length} 处</label><select id="booking-room">${rooms.map(x=>`<option value="${esc(x.id)}" ${String(x.id)===room?'selected':''}>${esc(x.campus)} · ${esc(x.name)}${x.status?'':'（停用）'}</option>`).join('')}</select></div><div><label for="booking-date">使用日期</label><input id="booking-date" type="date" value="${esc(date)}" min="${esc(today)}"></div></div><div class="actions">${button('查询空位','query',busy)}</div>`:''}
 <div role="status" aria-live="polite" class="notice" data-tone="${error?'error':'info'}">${esc(error||message||'校园网内可在这里直接查看场地空位，无需先登录。')}</div>
 ${rooms.length?bookingSlotsHTML(day):''}`}
 function card(){return `<section class="card campus-booking" id="booking-panel">${content()}</section>`}
 function paint(){const el=document.getElementById('booking-panel');if(el)el.innerHTML=content()}
 async function click(a){
  if(!['booking-rooms','booking-query'].includes(a))return false;
  if(busy)return true;
  const version=++requestVersion;busy=true;error='';message='正在读取学校场地信息…';day=null;paint();
  try{
   if(a==='booking-rooms'){
    rooms=[];day=null;
    const data=await fetchRooms();if(version!==requestVersion)return true;rooms=data.rooms;today=data.today;date=today;room=String(rooms.find(x=>x.status)?.id||rooms[0]?.id||'');
    message=rooms.length?`已从学校读取 ${rooms.length} 个场地`:'学校本次没有返回可查询的场地，请打开官方页面查看。';
   }else{
    day=null;
    const result=await api(`/api/booking/availability?room=${encodeURIComponent(room)}&date=${encodeURIComponent(date)}`);
    if(version!==requestVersion)return true;day=result;
    message='已读取学校空位。预约请点击「去学校页面预约」。';
   }
  }catch(e){if(version===requestVersion)error=e.message}
  finally{if(version===requestVersion){busy=false;paint()}}
  return true;
 }
 function change(e){
  if(!['booking-room','booking-date'].includes(e.target.id))return false;
  if(e.target.id==='booking-room')room=e.target.value;else date=e.target.value;
  requestVersion++;busy=false;day=null;error='';message='条件已更改，请重新查询空位';paint();return true;
 }
 return {card,click,change};
}

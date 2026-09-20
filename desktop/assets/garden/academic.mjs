export const CALENDAR_URL='https://www.szu.edu.cn/xxgk/xl.htm';
const day=86400000;
// The school calendar is a Shenzhen date, regardless of the computer's timezone.
export const schoolDate=(now=Date.now())=>new Date(now+8*3600000).toISOString().slice(0,10);
export function teachingWeek(terms,manual='',today=schoolDate()){
 if(manual){const diff=Math.floor((Date.parse(today)-Date.parse(manual))/day);return {title:diff<0?`距离第 1 周还有 ${-diff} 天`:`第 ${Math.floor(diff/7)+1} 周`,detail:'按手动设置的日期计算',manual:true}}
 const sorted=[...terms].sort((a,b)=>a.start.localeCompare(b.start));
 const term=sorted.find(t=>t.start<=today&&today<=t.end);
 if(term){const diff=Math.floor((Date.parse(today)-Date.parse(term.week_start))/day);return {title:diff<0?'开学报到期间':`第 ${Math.floor(diff/7)+1} 周`,detail:term.name,term}}
 const next=sorted.find(t=>t.start>today);
 return {title:next?'尚未进入下一学期':'当前不在已公布的学期内',detail:next?`${next.name} · ${next.start} 开始`:'等待学校发布下一学期校历',term:next};
}

export function createAcademicUI({getState,api,toast}){
 const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 let calendar=null,loading=false,error='';
 function summary(){
  const current=teachingWeek(calendar?.terms||[],getState().semester);
  return `<div class="card-head"><h2>本学期</h2><span class="badge">${current.manual?'手动日期':'官方校历'}</span></div><p class="academic-week">${!calendar&&!current.manual?'正在读取校历…':esc(current.title)}</p><p class="muted">${esc(current.detail)}</p>${current.term?`<p class="academic-dates">学期 ${esc(current.term.start)} — ${esc(current.term.end)}<br>第 1 周从 ${esc(current.term.week_start)} 开始，周日换周。</p>`:''}<p class="notice" role="status">${loading?'正在检查学校校历…':error?esc(error):calendar?.message?esc(calendar.message):calendar?`${calendar.stale?'上次校历 · ':''}${calendar.checked_at&&!calendar.checked_at.startsWith('0001')?'已检查 '+esc(new Date(calendar.checked_at).toLocaleString('zh-CN',{timeZone:'Asia/Shanghai',month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit'})):'等待联网检查'}；每天自动检查更新。`:'首次读取中'}</p>`;
 }
 function card(){return `<section class="card" id="academic-calendar"><div id="academic-summary">${summary()}</div><div class="actions"><button data-action="calendar-refresh" ${loading?'disabled':''}>更新校历</button><a class="button quiet" href="${CALENDAR_URL}" target="_blank" rel="noopener noreferrer">查看官方校历 ↗</a></div><small>显示全校校历周次；新生、补课及考试安排请以本人的教学通知为准。</small><details class="academic-manual"><summary>手动调整第 1 周</summary><form id="semester-form"><label for="semester">第 1 周起始日</label><input type="date" id="semester" name="semester" value="${esc(getState().semester)}" required><div class="actions"><button>保存日期</button><button type="button" data-action="calendar-auto">恢复自动校历</button></div></form></details></section>`}
 function paint(){const el=document.getElementById('academic-summary');if(el)el.innerHTML=summary();const b=document.querySelector('[data-action="calendar-refresh"]');if(b)b.disabled=loading}
 async function load(force=false){
  if(loading)return;
  loading=true;error='';paint();
  try{
   if(!calendar)calendar=await api('/api/campus/calendar');
   paint();
   if(force||calendar.stale||Date.now()-Date.parse(calendar.checked_at)>day){calendar=await api('/api/campus/calendar?refresh=1')}
   if(force)toast(calendar.stale?'未能更新校历，已保留可用日期':'官方校历已检查');
  }catch(e){error=e.message+'；可查看官方校历或手动设置'}finally{loading=false;paint()}
 }
 function timetable(){return `<section class="card span"><div class="card-head"><h2>我的课表</h2><span class="badge">学校系统查询</span></div><p class="muted">本科与研究生使用不同的选课系统。登录后查看个人课表，具体上课时间、调课与教室以学校系统为准。</p><div class="actions"><a class="button" href="https://ehall.szu.edu.cn/jwapp/sys/kcbcx/*default/index.do" target="_blank" rel="noopener noreferrer">本科课表 ↗</a><a class="button" href="https://ehall.szu.edu.cn/yjsxk" target="_blank" rel="noopener noreferrer">研究生选课与课表 ↗</a></div><small>当前打开官方页面查询。应用内课表同步尚未接入，成绩登录状态不代表课表业务已经可用。</small></section>`}
 return {card,load,timetable};
}

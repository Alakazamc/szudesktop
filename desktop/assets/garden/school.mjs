import {pixelIcon} from './pixel.mjs';
import {unverifiedBadge} from './labels.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const official='https://ehall.szu.edu.cn/yjsxk';
const schoolIcons={challenge:'i-key',read:'i-book',clear:'i-shield',undergrad:'i-book'};
const action=(label,key,extra='')=>`<button data-action="school-${key}" ${extra}>${pixelIcon(schoolIcons[key])}${label}</button>`;

export function timetableHTML(data){
 if(!data)return '<p class="empty">登录后点击「读取我的课表」，从学校系统获取当前学期安排。</p>';
 const days=['','周一','周二','周三','周四','周五','周六','周日'];
 const groups=days.slice(1).map((day,i)=>({day,entries:data.entries.filter(x=>x.day===i+1)})).filter(x=>x.entries.length);
 return `<p class="notice">${esc(data.round||data.term)} · 读取于 ${esc(new Date(data.fetched_at).toLocaleString('zh-CN'))}</p>${groups.length?`<div class="timetable-days">${groups.map(group=>`<section class="timetable-day"><h3>${group.day}</h3>${group.entries.map(x=>`<article class="timetable-course"><span class="badge" data-tone="info">第 ${x.start}${x.end!==x.start?'–'+x.end:''} 节</span><h4>${esc(x.name)}</h4><p>${esc(x.weeks||'周次以学校通知为准')}</p><p>${esc(x.room||'教室待定')}${x.teacher?' · '+esc(x.teacher):''}</p>${x.class?`<small>${esc(x.class)}</small>`:''}${x.scheme?`<small>${esc(x.scheme)}</small>`:''}</article>`).join('')}</section>`).join('')}</div>`:'<p class="empty">学校当前学期没有返回已排定的课程。</p>'}${data.unscheduled.length?`<h3>已选 · 待安排时间</h3><ul class="timetable-pending">${data.unscheduled.map(x=>`<li><strong>${esc(x.name)}</strong>${x.code?' · '+esc(x.code):''}</li>`).join('')}</ul>`:''}<small>当前接口提供选课系统的本学期课表。调课、考试和历史学期请在官方系统核对；课表仅在本次运行中显示，不写入庭院备份。</small>`;
}

export function createSchoolUI({api,toast}){
 let logged=false,challenge='',image='',message='',error='',data=null,busy=false,undergrad=null,undergradError='';
 function status(){return `<p role="status" data-tone="${error?'error':busy?'info':logged?'success':'muted'}" class="notice ${error?'error':logged?'ok':''}">${esc(error||message||(logged?'研究生教务已登录 · 关闭应用即清除':'尚未登录研究生教务。登录后到「我的课表」读取。'))}</p>`}
 function loginCard(){if(globalThis.szuDesktop?.openSchool)return '';return `<section class="card span" id="school-account"><div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-key','heading-icon')}研究生教务登录</h2>${unverifiedBadge()}</div><p class="muted">使用研究生选课系统的学号和密码。校园网登录与这里分开；不保存密码、不自动选课或退课。</p><div id="school-status">${status()}</div><details id="school-login-details" ${logged?'':'open'}><summary>${logged?'切换账号 / 重新登录':'登录研究生教务'}</summary><form id="school-login-form" autocomplete="off"><div class="grid"><div><label for="school-account-input">学号</label><input id="school-account-input" name="username" autocomplete="off" maxlength="80" required placeholder="输入学号，默认不回填"></div><div><label for="school-password">教务密码</label><input id="school-password" name="password" type="password" autocomplete="new-password" maxlength="128" required placeholder="仅用于本次登录"></div></div><div id="school-captcha">${captchaHTML()}</div><div class="actions"><button type="submit" class="primary" ${!challenge||busy?'disabled':''}>${pixelIcon('i-key')}登录教务</button>${action('获取 / 更换验证码','challenge','type="button"')}</div></form></details><div class="actions">${action('清除本次登录','clear',logged?'':'disabled')}<a class="button quiet" href="${official}" target="_blank" rel="noopener noreferrer">学校原页面 ↗</a></div></section>`}
 function timetableCard(){return `<section class="card span" id="school-timetable-card"><div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-book','heading-icon')}我的课表</h2>${unverifiedBadge()}</div><p class="muted">课表从学校系统读取，只在本次运行中显示、不写入庭院备份；调课、考试与历史学期请以官方系统为准。</p><h3>研究生 · 本学期课表</h3><div class="actions">${action('读取我的课表','read',logged?'':'disabled')}</div><p class="muted" id="school-timetable-hint">${logged?'已登录研究生教务，可以直接读取。':'需要先在上方完成研究生教务登录；读取的是当前选课学期。'}</p><div id="school-timetable" aria-live="polite">${timetableHTML(data)}</div><details><summary>本科 · 我的课表 ${unverifiedBadge()}</summary><p class="muted">本科使用统一身份认证。安装版请在上方选择「本科课表」，完成学校页面登录并读取本次登录；便携版可在成绩卡片使用备用登录。本科权限单独核验；研究生登录不能代替它。</p><div class="actions"><a class="button quiet" href="https://ehall.szu.edu.cn/jwapp/sys/wdkb/*default/index.do" target="_blank" rel="noopener noreferrer">本科我的课表 ↗</a></div><label for="undergrad-term">学期（留空则向学校查询当前学期）</label><input id="undergrad-term" maxlength="11" placeholder="例如 2026-2027-1"><div class="actions">${action('读取本科个人课表','undergrad')}</div><div id="undergrad-timetable" aria-live="polite">${undergradHTML()}</div><p class="muted">根据个人课表接口接入，尚待有本科权限的账号实测。保留学校返回的时间地点原文，暂不拆成每周网格。</p></details></section>`}
 function undergradHTML(){if(undergradError)return `<p class="notice error">${esc(undergradError)}</p>`;return undergradTimetableHTML(undergrad)}
 function captchaHTML(){return image?`<label for="school-verification">学校验证码</label><div class="school-captcha-row"><img src="${esc(image)}" width="160" height="50" alt="学校登录验证码"><input id="school-verification" name="captcha" maxlength="10" required autocomplete="off" placeholder="输入图片中的字符"></div>`:'<p class="muted">先获取学校验证码，再填写登录信息。</p>'}
 function paint(){const ug=document.getElementById('undergrad-timetable');if(ug)ug.innerHTML=undergradHTML();const el=document.getElementById('school-status');if(el)el.innerHTML=status();const result=document.getElementById('school-timetable');if(result)result.innerHTML=timetableHTML(data);for(const key of ['read','clear']){const b=document.querySelector(`[data-action="school-${key}"]`);if(b)b.disabled=(key==='clear'?(!logged&&!challenge):!logged)||busy}const hint=document.getElementById('school-timetable-hint');if(hint)hint.textContent=logged?'已登录研究生教务，可以直接读取。':'需要先在上方完成研究生教务登录；读取的是当前选课学期。';const submit=document.querySelector('#school-login-form button[type=submit]');if(submit)submit.disabled=!challenge||busy}
 async function load(){try{const result=await api('/api/academic/session');logged=result.authenticated;if(!logged)data=null;error=''}catch(e){logged=false;error=e.message}paint()}
 async function click(a){
  if(['campus-session-save','campus-session-clear'].includes(a)){undergrad=null;undergradError='';paint()}
  if(!a.startsWith('school-'))return false;
  if(a==='school-undergrad'){undergrad=null;undergradError='';paint();try{undergrad=await api('/api/academic/undergrad/timetable?term='+encodeURIComponent(document.getElementById('undergrad-term')?.value||''))}catch(e){undergradError=e.message}paint();return true;}
  busy=true;error='';paint();
  try{
   if(a==='school-challenge'){
    logged=false;data=null;challenge='';image='';message='正在向学校获取验证码…';paint();const box=document.getElementById('school-captcha');if(box)box.innerHTML=captchaHTML();
    const result=await api('/api/academic/challenge',{});challenge=result.challenge;image=result.image;message=result.message;
    const el=document.getElementById('school-captcha');if(el)el.innerHTML=captchaHTML();
   }else if(a==='school-read'){
    data=null;message='正在读取学校课表…';paint();data=await api('/api/academic/timetable');message='课表已从学校系统读取';
   }else if(a==='school-clear'){
    await api('/api/academic/session',undefined,'DELETE');logged=false;challenge='';image='';data=null;message='已清除本次教务登录';document.getElementById('school-login-form')?.reset();const el=document.getElementById('school-captcha');if(el)el.innerHTML=captchaHTML();
   }
  }catch(e){error=e.message;if(e.code===401||e.code===409)logged=false}
  finally{busy=false;paint()}
  return true;
 }
 async function submit(form,values){
  if(form.id!=='school-login-form')return false;
  busy=true;error='';data=null;message='正在登录学校教务…';paint();
  try{const result=await api('/api/academic/login',{...values,challenge});logged=!!result.authenticated;message=result.message;form.reset();const details=document.getElementById('school-login-details');if(details)details.open=false;toast('教务登录成功，可以读取课表了')}
  catch(e){logged=false;error=e.message}
  finally{values.password='';const pwd=document.getElementById('school-password');if(pwd)pwd.value='';challenge='';image='';const el=document.getElementById('school-captcha');if(el)el.innerHTML=captchaHTML();busy=false;paint()}
  return true;
 }
 function reset(){logged=false;data=null;undergrad=null;undergradError='';error='';message='';paint()}
 return {loginCard,timetableCard,load,click,submit,sync:paint,reset};
}

export function undergradTimetableHTML(data){
 if(!data)return '<p class="empty">本科课表尚未读取。</p>';
 return `<p class="notice">${esc(data.term)} · ${data.courses.length} 门课程 · ${esc(new Date(data.fetched_at).toLocaleString('zh-CN'))}</p>${data.courses.length?`<div class="timetable-days">${data.courses.map(c=>`<article class="timetable-course"><h4>${esc(c.name)}</h4><p>${esc(c.teacher||'教师未提供')}</p><p class="undergrad-arrangement">${esc(c.arrangement||'学校未提供时间地点')}</p><small>${esc(c.code)}${c.class?' · '+esc(c.class):''}</small></article>`).join('')}</div>`:'<p class="empty">学校该学期返回空课表，请结合学期和官方系统核对。</p>'}`;
}

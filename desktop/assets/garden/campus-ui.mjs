import {createNoticesUI} from './notices.mjs';
import {createBookingUI,createVenueRulesUI,createRoomsLoader} from './booking.mjs';
import {pixelIcon} from './pixel.mjs';
import {unverifiedBadge} from './labels.mjs';
import {parseGrades,mergeGrades,makeStudyReminder,reminderICS,BOOKING_URL,GRADE_RULE_URL,PHONE_BOOK,PHONE_FALLBACK,PHONE_NOTE} from './campus.mjs';
import {gpa} from './engine.mjs';

export function createCampusUI({getState,commit,toast,confirm,api,render}) {
 // 「学校场地列表」是两张卡片共用的同一次只读请求，共享一份带短 TTL 的结果。
 const loadRooms=createRoomsLoader(api);
 const booking=createBookingUI({api,loadRooms});
 const venueRules=createVenueRulesUI({api,loadRooms});
 const notices=createNoticesUI({api});
 const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const link=(url,label,cls='button')=>`<a class="${cls}" href="${esc(url)}" target="_blank" rel="noopener noreferrer">${label} ↗</a>`;
 const itemIcons={'reminder-ics':'i-calendar',feed:'i-bell','session-save':'i-chest','session-check':'i-shield','session-clear':'i-key','online-score':'i-medal',preview:'i-scroll',template:'i-scroll',import:'i-chest'};
 const button=(label,action,extra='')=>`<button data-action="campus-${action}" ${extra}>${itemIcons[action]?`<svg class="item-icon" aria-hidden="true"><use href="#${itemIcons[action]}"></use></svg>`:''}${label}</button>`;
 // 每张卡片各自兜底。services() 的返回值会被整段赋给 #main，只要其中一块抛异常，
 // 整页就静默不更新、也不报错。这里把失败收敛在单块卡片内。
 function safeCard(title,build){try{return build()}catch(e){return `<section class="card"><div class="card-head"><h2 class="icon-heading tone-warning">${pixelIcon('i-scroll','heading-icon')}${esc(title)}</h2><span class="badge" data-tone="warning">暂时不可用</span></div><p class="notice error">这一块没能显示（${esc(e&&e.message||'渲染异常')}）。其他内容不受影响，可以稍后重试。</p></section>`}}
 let gradeText='',gradeLevel='undergrad',preview=null,filterLevel='',filterTerm='';
 // 学校系统（ehall）在线读取相关状态。会话本身不放在这里，只由后端保管。
 let sessionSaved=false,sessionDesc='',sessionErr='',sessionBusy=false,onlineScore=null,onlineErr='',onlineBusy=false,onlineLevel='undergrad';
 // 统一身份认证（本科）应用内登录的状态。与上面粘 Cookie 是两条独立入口，
 // 后端优先用 CAS 会话，没有才回落到 Cookie。
 let casLogged=false,casErr='',casBusy=false,casChallenge='',casImage='',casNeedCaptcha=false;
 const formatTime=n=>new Date(n).toLocaleString('zh-CN',{month:'long',day:'numeric',hour:'2-digit',minute:'2-digit'});
 function download(text,name,type){const url=URL.createObjectURL(new Blob([text],{type})),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),2000)}
 function services(){const reminders=getState().reminders||[];return `${safeCard('学习空间 · 预约与空位',()=>booking.card())}
 ${safeCard('场地与琴房规则速查',()=>venueRules.card())}
 <section class="card campus-booking"><div class="card-head"><h2 class="icon-heading tone-info">${pixelIcon('i-mug','heading-icon')}图书馆与自习提醒</h2><span class="badge" data-tone="info">官方入口</span></div>
 <div class="actions">${link('https://webvpn.szu.edu.cn/','登录 WebVPN')}${link('https://www.lib.szu.edu.cn/space-and-facilities/discussion-room','图书馆研讨间')}</div>
 <p class="notice">图书馆研讨间与社区场地分属不同系统；阅览座位另按${link('https://www.lib.szu.edu.cn/space-and-facilities/seat','官方选座规则','')}签到选座。</p>
 <details><summary>添加自习提醒</summary><p class="muted">在官方系统确认预约后，可手动登记时间并导出日历。此处保存的是提醒，不会向学校提交预约。</p>
 <form id="campus-reminder-form" class="grid three"><div><label for="reminder-place">自习地点</label><input id="reminder-place" name="place" maxlength="80" placeholder="填写已预约的场地 / 房间" required></div><div><label for="reminder-start">开始时间</label><input id="reminder-start" name="start" type="datetime-local" required></div><div><label for="reminder-end">结束时间</label><input id="reminder-end" name="end" type="datetime-local" required></div><button>${pixelIcon('i-bell')}保存本机提醒</button></form></details>
 ${reminders.length?`<h3>自习提醒 · 手动登记</h3><ul class="campus-reminders">${[...reminders].sort((a,b)=>a.start-b.start).map(x=>`<li><div><strong>${esc(x.place)}</strong><p>${formatTime(x.start)} — ${formatTime(x.end)}${x.end<Date.now()?' · 已结束':''}</p></div><div class="actions">${button('导出日历','reminder-ics',`data-id="${esc(x.id)}"`)}${button('移除提醒','reminder-delete',`data-id="${esc(x.id)}"`)}</div></li>`).join('')}</ul><small>导入系统日历后可在开始前 15 分钟提醒；是否提醒由日历软件设置决定。</small>`:''}</section>
 ${safeCard('校园通知',()=>notices.card())}
 <section class="card campus-phone"><div class="card-head"><h2 class="icon-heading tone-magic">${pixelIcon('i-mail','heading-icon')}常用联系与入口</h2><span class="badge">公开信息</span></div>
 <p>${esc(PHONE_NOTE)}</p>
 <ul class="campus-phones">${PHONE_BOOK.map(x=>`<li><span>${esc(x.name)}</span>${x.tel?`<a href="tel:${esc(x.tel)}">${esc(x.tel)}</a>`:`<a href="mailto:${esc(x.mail)}">${esc(x.mail)}</a>`}${link(x.source,'来源','')}</li>`).join('')}</ul>
 <h3>其他部门 · 官方入口</h3>
 <div class="actions">${PHONE_FALLBACK.map(x=>link(x.url,x.name)).join('')}</div>
 <small>这些部门没有查到官方公开号码，因此只给入口：请在官方页面核对最新联系方式。</small></section>`}
 function sessionHTML(){
  if(sessionBusy)return '<p role="status">正在验证登录状态…</p>';
  if(sessionErr)return `<p role="status" class="notice error">${esc(sessionErr)}</p>`;
  if(!sessionSaved)return '<p class="muted">还没有保存学校系统登录状态。可按下面步骤保存，再验证所选成绩业务是否可访问。</p>';
  return `<p class="notice ok">已保存登录状态${sessionDesc?' · '+esc(sessionDesc):''}。内容加密保存在本机；查询时仅发送至学校办事大厅，不写入日志。已保存不代表业务验证通过。</p>`;
 }
 function casStatusHTML(){
  if(casBusy)return '<p role="status">正在处理…</p>';
  if(casErr)return `<p role="status" class="notice error">${esc(casErr)}</p>`;
  if(casLogged)return '<p role="status" class="notice ok">统一身份认证已登录 · 关闭应用即清除。本科课表与成绩会优先使用这条会话。</p>';
  return '<p class="muted">还没有登录统一身份认证。先点「获取 / 更换验证码」拿登录页，再填学号密码。</p>';
 }
 function casCaptchaHTML(){
  if(!casImage)return casNeedCaptcha?'<p class="muted">学校要求输入验证码，请先获取。</p>':'<p class="muted">本次登录可能不需要验证码；先获取登录页，按页面要求填写。</p>';
  return `<label for="cas-verification">学校验证码</label><div class="school-captcha-row"><img src="${esc(casImage)}" width="160" height="50" alt="学校登录验证码"><input id="cas-verification" name="captcha" maxlength="10" autocomplete="off" placeholder="输入图片中的字符"></div>`;
 }
 function refreshCasBox(){const el=document.querySelector('#cas-status');if(el)el.innerHTML=casStatusHTML();const box=document.querySelector('#cas-captcha');if(box)box.innerHTML=casCaptchaHTML();const submit=document.querySelector('#cas-login-form button[type=submit]');if(submit)submit.disabled=!casChallenge||casBusy}
 async function loadCas(){
  try{const v=await api('/api/cas/session');casLogged=!!v.authenticated;casErr=''}
  catch(e){casLogged=false;casErr=e.message}
  refreshCasBox();
 }
 function howtoHTML(){
  // 步骤写细一点：这一步对不熟开发者工具的同学是唯一的门槛。
  return `<details><summary>怎么拿到这段 Cookie</summary>
  <ol class="session-steps">
   <li>用浏览器打开 <a href="https://ehall.szu.edu.cn/" target="_blank" rel="noopener noreferrer">学校办事大厅 ↗</a> 并完成登录（该验证就验证，正常登录即可）。</li>
   <li>按 <kbd>F12</kbd> 打开开发者工具，切到「网络 / Network」标签页。</li>
   <li>先进入所需的本科或研究生成绩页面，再刷新；选取该成绩业务发往 <code>ehall.szu.edu.cn</code> 的请求。不同业务可能需要不同的登录状态。</li>
   <li>在右侧「标头 / Headers」里找到「请求标头 / Request Headers」中的 <code>Cookie</code>，把冒号后面的整串值复制下来。</li>
   <li>粘贴到下面输入框并点保存，选择本科或研究生，然后点「验证登录状态」。</li>
  </ol>
  <p class="muted">这段内容等同于你在这台电脑上的登录凭证。它只保存在本机（${sessionSaved?'已加密':'保存后加密'}），点「清除登录状态」只删除本机副本，不会注销浏览器或撤销学校会话；退出学校登录请在官方页面操作。安全存储不可用时会拒绝保存。请不要把它发给任何人，也不要粘到聊天窗口里。</p></details>`;
 }
 function onlineScoreHTML(){
  if(onlineBusy)return '<p role="status">正在读取学校系统…</p>';
  if(onlineErr)return `<p role="status" class="notice error">${esc(onlineErr)}</p>`;
  if(!onlineScore)return '<p class="muted">登录状态可用后，可以直接读取学校系统里的成绩，不用手工粘贴表格。</p>';
  const r=onlineScore;
  if(!Array.isArray(r.items))return '<p role="status" class="notice error">学校成绩响应格式异常，请到官方系统核对。</p>';
  return `<p class="notice">${esc(r.label)}成绩 · 读取 ${r.fetched} 门${Number.isInteger(r.total)?'（学校返回总计 '+r.total+' 条记录）':'（总数未确认）'}${r.note?' · '+esc(r.note):''}</p>
  ${r.items.length?`<div class="table-wrap"><table><thead><tr><th>课程</th><th>学期</th><th>学分</th><th>成绩</th><th>官方绩点</th></tr></thead><tbody>${r.items.map(x=>`<tr><td>${esc(x.name)}${x.category?'<small class="course-source">'+esc(x.category)+'</small>':''}</td><td>${esc(x.term||'—')}</td><td>${esc(x.credit??'—')}</td><td>${esc(x.score??'—')}</td><td>${esc(x.gpa??'未提供')}</td></tr>`).join('')}</tbody></table></div>`:'<p class="muted">本次查询返回空列表；是否存在其他成绩，请结合总数提示并到官方系统核对。</p>'}
  <p class="notice">这是从学校系统读到的原始记录，没有替你换算或补全。要并入上面的绩点统计，请用「批量导入成绩表」。</p>`;
 }
 function grades(){
  const courses=getState().courses,terms=[...new Set(courses.map(x=>x.term||'').filter(Boolean))].sort();
  const selected=courses.filter(x=>(!filterLevel||x.level===filterLevel)&&(!filterTerm||x.term===filterTerm)),result=gpa(selected);
  return `<section class="card span"><div class="card-head"><h2 class="icon-heading tone-magic">${pixelIcon('i-medal','heading-icon')}成绩与绩点</h2><span class="badge" data-tone="magic">计入 <b>${result.credits}</b> 学分 · 加权绩点 <b>${result.value.toFixed(2)}</b></span></div>
  <p>本科和研究生分开记录，批量导入课程后按学分加权。这里的结果用于个人核对，官方平均绩点以学校系统为准。</p>
  <div class="actions">${link('https://ehall.szu.edu.cn/','学校办事大厅')}${link('https://cjzm.szu.edu.cn/gztcyUI/','本科成绩证明')}${link('https://gra.szu.edu.cn/info/1092/3484.htm','研究生成绩单指南')}</div>
  <details open><summary>从学校系统直接读取（可选）${unverifiedBadge()}</summary>
  <p class="muted">在线读取尚未完成真实成绩验收。保存学校登录状态后，可尝试读取所选业务；账号权限和会话需分别验证。</p>
  ${globalThis.szuDesktop?.openSchool?'<p class="muted">在上方选择本科或研究生成绩，完成学校登录并读取本次登录，再点击下面的「读取成绩」。</p>':`<details><summary>便携版备用登录</summary>
  <div id="session-status" aria-live="polite">${sessionHTML()}</div>
  <div id="cas-status" aria-live="polite">${casStatusHTML()}</div>
  <form id="cas-login-form" autocomplete="off">
  <div class="grid">
   <div><label for="cas-username">学号</label><input id="cas-username" name="username" autocomplete="off" maxlength="80" required placeholder="统一身份认证的学号"></div>
   <div><label for="cas-password">密码</label><input id="cas-password" name="password" type="password" autocomplete="new-password" maxlength="128" required placeholder="仅用于本次登录，不保存"></div>
  </div>
  <div id="cas-captcha">${casCaptchaHTML()}</div>
  <div class="actions"><button type="submit" class="primary" ${casChallenge?'':'disabled'}>${pixelIcon('i-key')}登录统一身份认证</button>${button('获取学校登录页','cas-challenge','type="button"')}</div>
  </form>
  <p class="muted">用统一身份认证的学号密码直接登录，登完就能读本科课表和成绩，不用再去浏览器复制 Cookie。登录状态只留在本次运行里，关闭应用即清除。</p>
  <div class="actions">${button('清除本次登录','cas-clear',casLogged?'':'disabled')}</div>
  <hr>
  <p class="muted">或者用原来的办法：从浏览器复制 ehall 的 Cookie。</p>
  <label for="session-cookie">浏览器里的 Cookie</label>
  <textarea id="session-cookie" rows="3" maxlength="8000" placeholder="JSESSIONID=..."></textarea>
  <div class="actions">${button('保存登录状态','session-save','class="primary"')}${button('验证登录状态','session-check')}${button('清除登录状态','session-clear')}</div>
  ${howtoHTML()}
</details>`}
  <div class="actions" style="margin-top:10px"><label for="online-score-level">读取哪一份成绩</label><select id="online-score-level"><option value="undergrad" ${onlineLevel==='undergrad'?'selected':''}>本科</option><option value="graduate" ${onlineLevel==='graduate'?'selected':''}>研究生</option></select>${button('读取成绩','online-score')}</div>
  <div id="online-score" aria-live="polite">${onlineScoreHTML()}</div>
  <p class="notice">只查询成绩，不提交预约、选课或评教。登录状态过期时会明确报错，不会显示成「没有成绩」。</p></details>
  <details><summary>批量导入成绩表</summary><p class="muted">从学校成绩表或 Excel 复制包含表头的多行内容，或选择 CSV / TSV 文件。当前不直接解析 PDF、图片和 XLSX，也没有后台自动同步。</p>
  <label for="grade-level">这份成绩属于</label><select id="grade-level"><option value="undergrad" ${gradeLevel==='undergrad'?'selected':''}>本科</option><option value="graduate" ${gradeLevel==='graduate'?'selected':''}>研究生</option></select>
  <label for="grade-file">选择表格文件（可选）</label><input id="grade-file" type="file" accept=".csv,.tsv,.txt,text/csv,text/tab-separated-values,text/plain">
  <label for="grade-text">粘贴成绩表</label><textarea id="grade-text" rows="5" maxlength="300000" placeholder="课程名称&#9;学分&#9;绩点&#9;学期">${esc(gradeText)}</textarea>
  <div class="actions">${button('预览导入','preview','class="primary"')}${button('下载空白表头','template')}</div>
  <p class="notice">本科可识别 A+、A、B+、B、C+、C、D、F 等级，按${link(GRADE_RULE_URL,'学校规则','')}换算；已有官方绩点时优先使用。研究生只读取表中的官方绩点，不套用本科规则。重修、免修及不计绩点课程请核对后选择是否计入。</p>
  <div id="grade-preview" aria-live="polite">${previewHTML()}</div></details>
  <details><summary>补充一门课程</summary><form id="course-form" class="grid three"><div><label for="course-name">课程名称</label><input id="course-name" name="name" maxlength="100" required></div><div><label for="credit">学分</label><input id="credit" name="credit" type="number" min="0.1" max="100" step="0.1" required></div><div><label for="point">官方课程绩点</label><input id="point" name="point" type="number" min="0" max="5" step="0.01" required></div><div><label for="course-level">培养层次</label><select id="course-level" name="level"><option value="undergrad">本科</option><option value="graduate">研究生</option></select></div><div><label for="course-term">学期（可选）</label><input id="course-term" name="term" maxlength="40"></div><button>加入计算</button></form></details>
  <div class="actions"><label for="grade-filter-level">统计范围</label><select id="grade-filter-level"><option value="">全部层次</option><option value="undergrad" ${filterLevel==='undergrad'?'selected':''}>本科</option><option value="graduate" ${filterLevel==='graduate'?'selected':''}>研究生</option></select><label for="grade-filter-term">学期</label><select id="grade-filter-term"><option value="">全部学期</option>${terms.map(t=>`<option ${filterTerm===t?'selected':''}>${esc(t)}</option>`).join('')}</select></div>
  <div class="table-wrap"><table><thead><tr><th>课程 / 来源</th><th>层次 / 学期</th><th>学分</th><th>绩点</th><th>计入</th><th>操作</th></tr></thead><tbody>${courses.map((c,i)=>({c,i})).filter(({c})=>selected.includes(c)).map(({c,i})=>`<tr><td>${esc(c.name)}<small class="course-source">${esc(c.code||'')}${c.code?' · ':''}${esc(c.source||'手动录入')}</small></td><td>${c.level==='undergrad'?'本科':c.level==='graduate'?'研究生':'未分类'}<small class="course-source">${esc(c.term||'未填学期')}</small></td><td>${c.credit}</td><td>${c.point}</td><td>${button(c.included===false?'未计入':'已计入','course-toggle',`data-index="${i}" aria-pressed="${c.included!==false}"`)}</td><td><button class="quiet" data-action="courseDelete" data-index="${i}">移除</button></td></tr>`).join('')}</tbody></table></div>${!selected.length?'<p class="empty">这个范围内还没有课程。导入后会保存在本机。</p>':''}</section>`;
 }
 function previewHTML(){if(!preview)return '';return `<p>识别到 ${preview.courses.length} 门课程，${preview.errors.length} 行需要处理。相同课程代码（或名称）、学期、层次、学分和绩点的记录将跳过；不同成绩的重修记录保留。</p>${preview.errors.length?`<ul class="notice error">${preview.errors.map(x=>`<li>第 ${x.row} 行 ${esc(x.name)}：${esc(x.message)}</li>`).join('')}</ul><p>请修正以上行后重新预览，避免漏掉课程。</p>`:''}<div class="table-wrap"><table><thead><tr><th>课程</th><th>学期</th><th>学分</th><th>绩点</th></tr></thead><tbody>${preview.courses.slice(0,12).map(x=>`<tr><td>${esc(x.name)}</td><td>${esc(x.term)}</td><td>${x.credit}</td><td>${x.point}</td></tr>`).join('')}</tbody></table></div>${preview.courses.length>12?'<p>仅展示前 12 门，确认后导入全部有效课程。</p>':''}${button('确认合并到课程记录','import',preview.errors.length||!preview.courses.length?'disabled':'class="primary"')}`}
 async function click(action,b){
  if(await booking.click(action,b))return true;
  if(await venueRules.click(action))return true;
  if(await notices.click(action))return true;
  if(!action.startsWith('campus-'))return false;
  const a=action.slice(7);
  if(a==='session-save'){
   // 主动把输入框清掉：这段内容留在页面上没有任何好处。
   const box=document.querySelector('#session-cookie'),raw=box?box.value:'';
   sessionErr='';sessionBusy=true;refreshSessionBox();
   try{await api('/api/session',{cookie:raw},'POST');if(box)box.value='';onlineScore=null;onlineErr='';refreshScoreBox();sessionSaved=true;await loadSession();toast('登录状态已保存到本机');}
   catch(e){sessionErr=e.message}
   finally{sessionBusy=false;refreshSessionBox()}
  }else if(a==='session-check'){
   sessionErr='';sessionBusy=true;refreshSessionBox();
   try{const r=await api('/api/session/check?level='+encodeURIComponent(onlineLevel),{});toast(r.message||'登录状态可用')}
   catch(e){sessionErr=e.message}
   finally{sessionBusy=false;refreshSessionBox()}
  }else if(a==='session-clear'){
   if(await confirm('清除学校系统登录状态？','只清除本机保存的登录状态，不影响你的校园网账号密码，也不会退出浏览器里的登录。清除后需要重新复制一次 Cookie。')){
    try{await api('/api/session',{},'DELETE');const box=document.querySelector('#session-cookie');if(box)box.value='';sessionSaved=false;sessionErr='';onlineScore=null;onlineErr='';toast('已清除本机保存的登录状态')}
    catch(e){toast(e.message)}
    refreshSessionBox();refreshScoreBox();
   }
  }else if(a==='cas-challenge'){
   casErr='';casBusy=true;casChallenge='';casImage='';casNeedCaptcha=false;refreshCasBox();
   try{
    const r=await api('/api/cas/challenge',{});
    casChallenge=r.challenge;casImage=r.image||'';casNeedCaptcha=!!r.image;
    const box=document.querySelector('#cas-captcha');if(box)box.innerHTML=casCaptchaHTML();
    const submit=document.querySelector('#cas-login-form button[type=submit]');if(submit)submit.disabled=false;
    toast(r.message||'请填写学号密码登录');
   }catch(e){casErr=e.message;casChallenge='';casImage=''}
   finally{casBusy=false;refreshCasBox()}
  }else if(a==='cas-clear'){
   if(await confirm('清除统一身份认证登录？','只清除本次运行的登录状态，不影响你的校园网账号密码，也不影响浏览器里的登录。清除后需要重新登录。')){
    try{await api('/api/cas/session',{},'DELETE');casLogged=false;casChallenge='';casImage='';onlineScore=null;onlineErr='';toast('已清除统一身份认证登录')}
    catch(e){toast(e.message)}
    refreshCasBox();refreshScoreBox();
   }
  }else if(a==='online-score'){
   const level=onlineLevel;
   onlineErr='';onlineScore=null;onlineBusy=true;refreshScoreBox();
   try{onlineScore=await api('/api/scores?level='+encodeURIComponent(level))}
   catch(e){onlineErr=e.message;if(e.code===401){sessionErr=e.message;casLogged=false;refreshSessionBox();refreshCasBox()}}
   finally{onlineBusy=false;refreshScoreBox()}
  }else if(a==='preview'){preview=parseGrades(gradeText,gradeLevel);document.querySelector('#grade-preview').innerHTML=previewHTML()}
  else if(a==='template')download('\uFEFF课程名称,学分,绩点,成绩,学期,课程代码\r\n','成绩表-空白表头.csv','text/csv;charset=utf-8');
  else if(a==='import'){
   if(!preview||preview.errors.length||!preview.courses.length)throw Error('请先修正成绩表并预览');
   const next=structuredClone(getState()),merged=mergeGrades(next.courses,preview.courses);next.courses=merged.courses;
   await commit(next);preview=null;gradeText='';render();toast(`课程已保存，跳过 ${merged.duplicates} 条重复记录`);
  }else if(a==='course-toggle'){const next=structuredClone(getState());next.courses[Number(b.dataset.index)].included=next.courses[Number(b.dataset.index)].included===false;await commit(next)}
  else if(a==='reminder-ics'){const item=getState().reminders.find(x=>x.id===b.dataset.id);if(item)download(reminderICS(item),'自习提醒.ics','text/calendar;charset=utf-8')}
  else if(a==='reminder-delete'){if(await confirm('移除本机提醒？','只移除这里的提醒，不会取消学校系统中的预约，也不会删除已经导入日历的事件。')){const next=structuredClone(getState());next.reminders=next.reminders.filter(x=>x.id!==b.dataset.id);await commit(next)}}
  return true;
 }
 function refreshSessionBox(){const el=document.querySelector('#session-status');if(el)el.innerHTML=sessionHTML()}
 function refreshScoreBox(){const el=document.querySelector('#online-score');if(el)el.innerHTML=onlineScoreHTML()}
 async function loadSession(){
  try{const v=await api('/api/session');sessionSaved=!!v.saved;sessionDesc=v.store_desc||'';sessionErr=''}
  catch(e){sessionSaved=false;sessionDesc='';sessionErr=e.message}
  refreshSessionBox();
 }
 async function submit(form,values){
  if(form.id==='cas-login-form'){
   casErr='';casBusy=true;refreshCasBox();
   try{
    const r=await api('/api/cas/login',{...values,challenge:casChallenge});
    casLogged=!!r.authenticated;casChallenge='';casImage='';casNeedCaptcha=false;
    form.reset();const box=document.querySelector('#cas-captcha');if(box)box.innerHTML=casCaptchaHTML();
    onlineScore=null;onlineErr='';
    toast(r.message||'登录成功，可以读取本科课表和成绩了');
   }catch(e){casErr=e.message;casLogged=false}
   finally{values.password='';const pwd=document.getElementById('cas-password');if(pwd)pwd.value='';casBusy=false;refreshCasBox()}
   return true;
  }
  if(form.id!=='campus-reminder-form')return false;const next=structuredClone(getState());next.reminders=next.reminders||[];if(next.reminders.length>=50)throw Error('最多保留 50 条提醒，请先移除已结束的提醒');next.reminders.push(makeStudyReminder(values));await commit(next,{formId:form.id,values});toast('已保存本机提醒，学校预约状态不变');return true}
 function input(e){if(e.target.id==='grade-text'){gradeText=e.target.value;preview=null;const el=document.querySelector('#grade-preview');if(el)el.innerHTML=''}}
 async function change(e){if(booking.change(e))return;if(await notices.change(e))return;const el=e.target;
  if(el.id==='online-score-level'){onlineLevel=el.value;onlineScore=null;onlineErr='';sessionErr='';refreshScoreBox();refreshSessionBox()}
  else if(el.id==='grade-level'){gradeLevel=el.value;preview=null;document.querySelector('#grade-preview').innerHTML=''}
  else if(el.id==='grade-file'&&el.files[0]){const file=el.files[0];if(file.size>300000)throw Error('文件不能超过 300 KB');if(!/\.(csv|tsv|txt)$/i.test(file.name))throw Error('请使用 CSV / TSV / TXT 文件');const bytes=await file.arrayBuffer();try{gradeText=new TextDecoder('utf-8',{fatal:true}).decode(bytes)}catch{gradeText=new TextDecoder('gb18030',{fatal:true}).decode(bytes)}preview=null;document.querySelector('#grade-text').value=gradeText;document.querySelector('#grade-preview').innerHTML='';toast('文件已读取，请点击预览导入')}
  else if(el.id==='grade-filter-level'){filterLevel=el.value;render()}
  else if(el.id==='grade-filter-term'){filterTerm=el.value;render()}
 }
 function resetSchoolData(){onlineScore=null;onlineErr='';refreshScoreBox()}
 return {services,grades,click,submit,input,change,loadSession,loadCas,loadSources:notices.load,resetSchoolData};
}

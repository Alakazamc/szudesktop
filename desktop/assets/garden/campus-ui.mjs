import {parseGrades,mergeGrades,makeStudyReminder,reminderICS,BOOKING_URL,GRADE_RULE_URL} from './campus.mjs';
import {gpa} from './engine.mjs';

export function createCampusUI({getState,commit,toast,confirm,api,render}) {
 const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const link=(url,label,cls='button')=>`<a class="${cls}" href="${esc(url)}" target="_blank" rel="noopener noreferrer">${label} ↗</a>`;
 const button=(label,action,extra='')=>`<button data-action="campus-${action}" ${extra}>${label}</button>`;
 let gradeText='',gradeLevel='undergrad',preview=null,feedSource='undergrad',feed=null,feedError='',loading=false,filterLevel='',filterTerm='';
 const formatTime=n=>new Date(n).toLocaleString('zh-CN',{month:'long',day:'numeric',hour:'2-digit',minute:'2-digit'});
 function download(text,name,type){const url=URL.createObjectURL(new Blob([text],{type})),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),2000)}
 function feedHTML(){
  if(loading)return '<p role="status">正在读取学校公开公告…</p>';
  if(feedError)return `<p role="status" class="notice error">${esc(feedError)}</p>`;
  if(!feed)return '<p class="muted">读取学校公开页面的公告标题与日期。点击上方按钮获取，不需要登录。</p>';
  const sorted=[...feed.items].sort((a,b)=>b.date.localeCompare(a.date));
  return `<p class="notice">${esc(feed.source)} · ${feed.stale?'上次读取的内容':'读取于 '+formatTime(feed.fetched_at)}${feed.message?' · '+esc(feed.message):''}</p><ul class="campus-notices">${sorted.slice(0,12).map(x=>`<li><time>${esc(x.date)}</time>${link(x.url,esc(x.title),'')}</li>`).join('')}</ul>`;
 }
 function services(){const reminders=getState().reminders||[];return `
 <section class="card campus-booking"><div class="card-head"><h2>学习空间与场地预约</h2><span class="badge">社区 · 图书馆</span></div>
 <p>社区静音舱包含共享会议室、网络面试间、琴房等空间；请按场地规定的用途预约。本科生与研究生从学校系统登录，具体可预约范围以账号权限为准。</p>
 <div class="actions">${link(BOOKING_URL,'打开静音舱预约','button primary')}${link('https://webvpn.szu.edu.cn/','登录 WebVPN')}${link('https://www.lib.szu.edu.cn/space-and-facilities/discussion-room','图书馆研讨间')}</div>
 <p class="notice">当前预约在学校页面完成；应用尚未读取空闲时段或预约结果。登录后若回到大厅，请再次点击「打开静音舱预约」。图书馆阅览座位另按${link('https://www.lib.szu.edu.cn/space-and-facilities/seat','官方选座规则','')}签到选座。</p>
 <details><summary>添加自习提醒</summary><p class="muted">在官方系统确认预约后，可手动登记时间并导出日历。此处保存的是提醒，不会向学校提交预约。</p>
 <form id="campus-reminder-form" class="grid three"><div><label for="reminder-place">自习地点</label><input id="reminder-place" name="place" maxlength="80" placeholder="填写已预约的场地 / 房间" required></div><div><label for="reminder-start">开始时间</label><input id="reminder-start" name="start" type="datetime-local" required></div><div><label for="reminder-end">结束时间</label><input id="reminder-end" name="end" type="datetime-local" required></div><button>保存本机提醒</button></form></details>
 ${reminders.length?`<h3>自习提醒 · 手动登记</h3><ul class="campus-reminders">${[...reminders].sort((a,b)=>a.start-b.start).map(x=>`<li><div><strong>${esc(x.place)}</strong><p>${formatTime(x.start)} — ${formatTime(x.end)}${x.end<Date.now()?' · 已结束':''}</p></div><div class="actions">${button('导出日历','reminder-ics',`data-id="${esc(x.id)}"`)}${button('移除提醒','reminder-delete',`data-id="${esc(x.id)}"`)}</div></li>`).join('')}</ul><small>导入系统日历后可在开始前 15 分钟提醒；是否提醒由日历软件设置决定。</small>`:''}</section>
 <section class="card campus-feed"><div class="card-head"><h2>学校公告</h2><span class="badge">官方公开内容</span></div><div class="actions"><label for="feed-source">来源</label><select id="feed-source"><option value="undergrad" ${feedSource==='undergrad'?'selected':''}>本科 · 教务部</option><option value="graduate" ${feedSource==='graduate'?'selected':''}>研究生院</option></select>${button('读取公告','feed')}${link(feedSource==='undergrad'?'https://jwb.szu.edu.cn/index/jwtz.htm':'https://gra.szu.edu.cn/','查看原页')}</div><div id="campus-feed-content" aria-live="polite">${feedHTML()}</div><small>10 分钟内复用已读取内容；公告按学校页面日期展示，原文以学校发布为准。</small></section>`}
 function grades(){
  const courses=getState().courses,terms=[...new Set(courses.map(x=>x.term||'').filter(Boolean))].sort();
  const selected=courses.filter(x=>(!filterLevel||x.level===filterLevel)&&(!filterTerm||x.term===filterTerm)),result=gpa(selected);
  return `<section class="card span"><div class="card-head"><h2>成绩与绩点</h2><span class="badge">计入 ${result.credits} 学分 · 加权绩点 ${result.value.toFixed(2)}</span></div>
  <p>本科和研究生分开记录，批量导入课程后按学分加权。这里的结果用于个人核对，官方平均绩点以学校系统为准。</p>
  <div class="actions">${link('https://ehall.szu.edu.cn/','学校办事大厅')}${link('https://cjzm.szu.edu.cn/gztcyUI/','本科成绩证明')}${link('https://gra.szu.edu.cn/info/1092/3484.htm','研究生成绩单指南')}</div>
  <details open><summary>批量导入成绩表</summary><p class="muted">从学校成绩表或 Excel 复制包含表头的多行内容，或选择 CSV / TSV 文件。当前不直接解析 PDF、图片和 XLSX，也没有后台自动同步。</p>
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
  if(!action.startsWith('campus-'))return false;
  const a=action.slice(7);
  if(a==='feed'){
   loading=true;feedError='';document.querySelector('#campus-feed-content').innerHTML=feedHTML();
   try{feed=await api('/api/campus/notices?source='+feedSource)}catch(e){feedError=e.message}finally{loading=false;const el=document.querySelector('#campus-feed-content');if(el)el.innerHTML=feedHTML()}
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
 async function submit(form,values){if(form.id!=='campus-reminder-form')return false;const next=structuredClone(getState());next.reminders=next.reminders||[];if(next.reminders.length>=50)throw Error('最多保留 50 条提醒，请先移除已结束的提醒');next.reminders.push(makeStudyReminder(values));await commit(next,{formId:form.id,values});toast('已保存本机提醒，学校预约状态不变');return true}
 function input(e){if(e.target.id==='grade-text'){gradeText=e.target.value;preview=null;const el=document.querySelector('#grade-preview');if(el)el.innerHTML=''}}
 async function change(e){const el=e.target;
  if(el.id==='grade-level'){gradeLevel=el.value;preview=null;document.querySelector('#grade-preview').innerHTML=''}
  else if(el.id==='grade-file'&&el.files[0]){const file=el.files[0];if(file.size>300000)throw Error('文件不能超过 300 KB');if(!/\.(csv|tsv|txt)$/i.test(file.name))throw Error('请使用 CSV / TSV / TXT 文件');const bytes=await file.arrayBuffer();try{gradeText=new TextDecoder('utf-8',{fatal:true}).decode(bytes)}catch{gradeText=new TextDecoder('gb18030',{fatal:true}).decode(bytes)}preview=null;document.querySelector('#grade-text').value=gradeText;document.querySelector('#grade-preview').innerHTML='';toast('文件已读取，请点击预览导入')}
  else if(el.id==='feed-source'){feedSource=el.value;feed=null;feedError='';render()}
  else if(el.id==='grade-filter-level'){filterLevel=el.value;render()}
  else if(el.id==='grade-filter-term'){filterTerm=el.value;render()}
 }
 return {services,grades,click,submit,input,change};
}

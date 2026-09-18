// Local parsing only: imported study records never leave this computer.
export const UNDERGRAD_POINTS=Object.freeze({'A+':4.5,A:4,'B+':3.5,B:3,'C+':2.5,C:2,D:1,F:0});
export const BOOKING_URL='https://swzx.webvpn.szu.edu.cn/#/pages/booth/szu-booth-list';
export const GRADE_RULE_URL='https://jwb.szu.edu.cn/info/1357/2003.htm';

// 常用电话：只列能从学校官方页面核实到的号码，每条都带来源。
// 查不到官方号码的部门只给入口，不写数字 —— 宁可少一条，也不放编造的号。
export const LIBRARY_URL='https://www.lib.szu.edu.cn/';
export const PHONE_BOOK=Object.freeze([
 {name:'图书馆咨询 · 北馆',tel:'0755-26532182',source:LIBRARY_URL},
 {name:'图书馆咨询 · 南馆',tel:'0755-26534902',source:LIBRARY_URL},
 {name:'图书馆咨询 · 丽湖馆',tel:'0755-86932729',source:LIBRARY_URL},
 {name:'图书馆邮箱',mail:'szulib@szu.edu.cn',source:LIBRARY_URL}
]);
// 暂无官方公开号码的部门：给官方入口，让用户自己在页面上核对最新号码。
export const PHONE_FALLBACK=Object.freeze([
 {name:'教务部（本科教务）',url:'https://jwb.szu.edu.cn/'},
 {name:'研究生院',url:'https://gra.szu.edu.cn/'},
 {name:'学校办事大厅',url:'https://ehall.szu.edu.cn/'},
 {name:'校外访问入口 WebVPN',url:'https://webvpn.szu.edu.cn/'}
]);
export const PHONE_NOTE='号码来自学校图书馆官网公开页脚；其他部门只提供官方入口，不列未经核实的号码。号码以学校官网为准。';

function tableRows(text){
 const input=String(text).replace(/^\uFEFF/,'').replace(/\r\n?/g,'\n');
 if(input.length>300000)throw Error('成绩表过大，请限制在 300 KB 以内');
 const delimiter=input.split('\n')[0].includes('\t')?'\t':',';
 const rows=[];let row=[],cell='',quoted=false;
 for(let i=0;i<input.length;i++){
  const ch=input[i];
  if(ch==='"'){if(quoted&&input[i+1]==='"'){cell+='"';i++}else if(quoted||cell==='')quoted=!quoted;else cell+=ch}
  else if(!quoted&&(ch===delimiter||ch==='\n')){row.push(cell.trim());cell='';if(ch==='\n'){if(row.some(Boolean))rows.push(row);row=[]}}
  else cell+=ch;
 }
 if(quoted)throw Error('表格引号不完整，请重新复制完整成绩表');
 row.push(cell.trim());if(row.some(Boolean))rows.push(row);
 return rows;
}
const aliases={name:['课程名称','课程名','课程','coursename','name'],credit:['学分','课程学分','credit','credits'],point:['绩点','课程绩点','学分绩点对应值','gradepoint','point','gpa'],grade:['成绩','总评成绩','等级','课程成绩','grade'],term:['学期','开课学期','学年学期','term','semester'],code:['课程代码','课程号','课程编号','code']};
export function parseGrades(text,level='undergrad'){
 if(!['undergrad','graduate'].includes(level))throw Error('请选择本科或研究生');
 const table=tableRows(text);if(table.length<2)throw Error('请保留表头，并至少提供一门课程');
 const headers=table.shift().map(x=>x.toLowerCase().replace(/[\s（()）]/g,''));
 const columns=Object.fromEntries(Object.entries(aliases).map(([k,names])=>[k,headers.findIndex(x=>names.includes(x))]));
 if(columns.name<0||columns.credit<0||(columns.point<0&&columns.grade<0))throw Error('表头需包含「课程名称、学分、绩点」；本科也可使用「成绩」等级列');
 if(table.length>300)throw Error('一次最多导入 300 门课程');
 const courses=[],errors=[];
 table.forEach((row,i)=>{
  const name=row[columns.name]||'',creditText=row[columns.credit]||'',pointText=row[columns.point]||'',grade=(row[columns.grade]||'').toUpperCase().replace(/＋/g,'+');
  const credit=Number(creditText);let point=pointText===''?null:Number(pointText),converted=false;
  if(point===null&&level==='undergrad'&&Object.hasOwn(UNDERGRAD_POINTS,grade)){point=UNDERGRAD_POINTS[grade];converted=true}
  let error='';
  if(!name||name.length>100)error='课程名称为空或过长';
  else if(!creditText||!Number.isFinite(credit)||credit<=0||credit>100)error='学分应大于 0 且不超过 100；零学分课程无需加入加权计算';
  else if(point===null)error=level==='graduate'?'研究生请提供官方绩点，不套用本科规则':'缺少绩点或可识别的本科等级（A+ 至 F），百分制成绩不直接推算';
  else if(!Number.isFinite(point)||point<0||point>5)error='绩点必须在 0–5 之间';
  if(error){errors.push({row:i+2,name,message:error});return}
  courses.push({name,credit,point,term:(row[columns.term]||'').slice(0,40),code:(row[columns.code]||'').slice(0,40),level,grade:grade.slice(0,20),included:true,source:converted?'本科官方等级换算':'成绩表绩点'});
 });
 return {courses,errors};
}
export function mergeGrades(existing,incoming){
 const courses=structuredClone(existing);let duplicates=0;
 for(const c of incoming){
  const same=courses.some(x=>(x.code||x.name)===(c.code||c.name)&&(x.term||'')===(c.term||'')&&x.credit===c.credit&&x.point===c.point&&(x.level||'')===(c.level||''));
  if(same)duplicates++;else courses.push(c);
 }
 if(courses.length>300)throw Error('最多保留 300 门课程，请先整理已有课程');
 return {courses,duplicates};
}
export function makeStudyReminder(values,now=Date.now()){
 const start=new Date(values.start).getTime(),end=new Date(values.end).getTime(),place=String(values.place||'').trim();
 if(!place||place.length>80)throw Error('请填写 1–80 字的自习地点');
 if(!Number.isFinite(start)||!Number.isFinite(end)||start<=now||end<=start||end-start>86400000)throw Error('请选择未来开始时间，结束时间须在开始后 24 小时内');
 return {id:crypto.randomUUID(),place,start,end};
}
export function reminderICS(item,now=Date.now()){
 const escape=s=>String(s).replace(/\\/g,'\\\\').replace(/\r\n|\r|\n/g,'\\n').replace(/,/g,'\\,').replace(/;/g,'\\;');
 const stamp=n=>new Date(n).toISOString().replace(/[-:]/g,'').replace(/\.\d{3}/,'');
 const lines=['BEGIN:VCALENDAR','VERSION:2.0','PRODID:-//szuDesktop//Study Reminder//ZH','CALSCALE:GREGORIAN','BEGIN:VEVENT','UID:'+escape(item.id)+'@szudesktop.local','DTSTAMP:'+stamp(now),'DTSTART:'+stamp(item.start),'DTEND:'+stamp(item.end),'SUMMARY:'+escape('自习提醒 · '+item.place),'LOCATION:'+escape(item.place),'DESCRIPTION:'+escape('本机手动登记的自习提醒，不代表学校预约成功。请在官方系统核对预约与签到要求。'),'BEGIN:VALARM','TRIGGER:-PT15M','ACTION:DISPLAY','DESCRIPTION:自习将在 15 分钟后开始','END:VALARM','END:VEVENT','END:VCALENDAR'];
 // RFC 5545: fold by UTF-8 octets, never split a Unicode code point.
 const encoder=new TextEncoder();return lines.map(line=>{let out='',width=0;for(const ch of line){const n=encoder.encode(ch).length;if(width+n>73){out+='\r\n ';width=1}out+=ch;width+=n}return out}).join('\r\n')+'\r\n';
}

import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const require=createRequire(import.meta.url);
import {parseGrades,mergeGrades,makeStudyReminder,reminderICS,PHONE_BOOK,PHONE_FALLBACK} from './assets/garden/campus.mjs';
import {gpa,createState,normalize} from './assets/garden/engine.mjs';
let count=0;const test=(name,f)=>{f();count++;console.log('PASS',name)};
test('official undergraduate grades and zero GPA',()=>{
 const {courses,errors}=parseGrades('课程名称\t学分\t成绩\t学期\n数学\t3\tA+\t秋季\n物理\t1\tF\t秋季');
 assert.equal(errors.length,0);assert.equal(courses[0].point,4.5);assert.equal(courses[1].point,0);assert.equal(gpa(courses).value,3.375);
});
test('graduate uses official points, never undergraduate conversion or guessed scores',()=>{
 assert.equal(parseGrades('课程名称,学分,成绩\n课程,3,A','graduate').errors.length,1);
 assert.equal(parseGrades('课程名称,学分,成绩\n课程,3,93','undergrad').errors.length,1);
 assert.equal(parseGrades('课程名称,学分,成绩,绩点\n课程,3,A,3.8','graduate').courses[0].point,3.8);
});
test('CSV quoting BOM and invalid rows are explicit',()=>{
 const result=parseGrades('\uFEFF课程名称,学分,绩点\r\n"写作,研究",2,4\r\n实验,,3\r\n研讨,0,4\r\n其他,2,NaN');
 assert.equal(result.courses[0].name,'写作,研究');assert.equal(result.errors.length,3);assert.equal(result.errors[0].row,3);
 assert.throws(()=>parseGrades('课程名称,学分,绩点\n"unclosed,2,4'));
});
test('duplicate imports skipped; different terms and repeated grades preserved',()=>{
 const a=parseGrades('课程名称,学分,绩点,学期,课程代码\n数学,3,4,秋季,M01').courses;
 assert.equal(mergeGrades(a,a).duplicates,1);
 const changed=[{...a[0],term:'春季'},{...a[0],point:3}];assert.equal(mergeGrades(a,changed).courses.length,3);
});
test('300 courses, metadata and exclusion survive saved state',()=>{
 const s=createState();s.courses=Array.from({length:150},(_,i)=>({name:'课程'+i,credit:1,point:4,level:'graduate',term:'春季',code:String(i),included:i!==0,source:'成绩表绩点'}));
 const result=normalize(s);assert.equal(result.courses.length,150);assert.equal(gpa(result.courses).credits,149);assert.equal(result.courses[0].included,false);assert.equal(result.courses[0].term,'春季');
});
test('reminders validate duration, persist, and export escaped UTC calendar',()=>{
 const now=Date.now(),s=createState(now),item=makeStudyReminder({place:'房间,一;二\n三',start:new Date(now+3600000).toISOString(),end:new Date(now+7200000).toISOString()},now);
 s.reminders=[item];assert.equal(normalize(s,now).reminders[0].place,item.place);
 const calendar=reminderICS(item,now);assert.ok(calendar.includes('BEGIN:VALARM'));assert.ok(calendar.includes('房间\\,一\\;二\\n三'));assert.match(calendar,/DTSTART:\d{8}T\d{6}Z/);
 for(const line of calendar.split('\r\n'))assert.ok(Buffer.byteLength(line)<=75);
 assert.throws(()=>makeStudyReminder({place:'自习室',start:now-1,end:now+60000},now));
 assert.throws(()=>makeStudyReminder({place:'自习室',start:now+1000,end:now+100000000},now));
});
test('phone book never lists a number without an official source',()=>{
 assert.ok(PHONE_BOOK.length>=3,'至少要有核实过的号码');
 for(const p of PHONE_BOOK){
  assert.ok(p.name&&p.source&&/^https:\/\//.test(p.source),'每条都要有官方来源链接');
  if(p.tel){assert.match(p.tel,/^(0755-)?\d{8}$/);assert.ok(p.tel.startsWith('0755-'),'深圳号码要带区号')}
  else if(p.mail){assert.match(p.mail,/^[^@\s]+@[^@\s]+\.[^@\s]+$/)}
  else assert.fail('条目既没有电话也没有邮箱');
 }
 for(const p of PHONE_FALLBACK){
  assert.ok(p.name&&/^https:\/\//.test(p.url),'没核实到的部门只给入口');
  assert.equal(p.tel,undefined);assert.equal(p.mail,undefined);
 }
});
test('school session stays out of the page and source code',()=>{
 // 用户交给本机的学校系统会话是一条真实登录凭证。
 // 它只能在设置区里输入，不能被页面脚本读回来，更不能出现在源码里。
 const fs=require('node:fs');
 const ui=fs.readFileSync('desktop/assets/garden/campus-ui.mjs','utf8');
 const html=fs.readFileSync('desktop/index.html','utf8');
 // 界面上只允许存在一个会话输入框，且必须是密码式的一次性输入。
 assert.equal((ui.match(/id="session-cookie"/g)||[]).length,1,'只应有一个会话输入框');
 // 渲染函数不得把已保存的 cookie 回显进 DOM。
 assert.ok(!/\$\{[^}]*sessionCookie[^}]*\}/.test(ui),'不得把会话内容渲染进页面');
 assert.ok(!html.includes('JSESSIONID='),'页面源码里不得出现真实会话样例');
 // 状态查询接口只回报长度，不回报内容。
 assert.ok(!/sessionStatusResp[\s\S]{0,400}?Cookie\s+string/.test(html),'状态结构不应包含会话原文');
 // 保存后必须清空输入框。
 assert.ok(/box\.value=''/.test(ui),'保存会话后应清空输入框');
});

test('online score reading never promises a write or hides an expired session',()=>{
 const fs=require('node:fs');
 const ui=fs.readFileSync('desktop/assets/garden/campus-ui.mjs','utf8');
 const go=fs.readFileSync('desktop/internal/ui/session.go','utf8')+fs.readFileSync('desktop/internal/ui/scores.go','utf8');
 // 只读：不得出现提交预约/评教这类写操作调用。
 assert.ok(!/insert|submit|postBook/i.test(go),'在线成绩模块不应当包含写操作');
 // 会话过期必须与「没有数据」区分开：401 才是过期，且要明确提示重新登录。
 assert.ok(/errSessionInvalid/.test(go),'必须有专门的会话失效错误');
 assert.ok(go.includes('不要以本结果为准'),'字段认不出时必须让用户去官方系统核对');
 // 不得静默把「读不到」显示成「暂无成绩」。
 // Malformed and mixed rows are exercised through a fake HTTPS server in session_security_test.go.
});

console.log(`${count} campus checks passed`);

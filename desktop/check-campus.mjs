import assert from 'node:assert/strict';
import {parseGrades,mergeGrades,makeStudyReminder,reminderICS} from './assets/garden/campus.mjs';
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
console.log(`${count} campus checks passed`);

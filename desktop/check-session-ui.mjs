import assert from 'node:assert/strict';
import {createCampusUI} from './assets/garden/campus-ui.mjs';
import {createState} from './assets/garden/engine.mjs';
const nodes=new Map();
const node=id=>{if(!nodes.has(id))nodes.set(id,{innerHTML:'',value:''});return nodes.get(id)};
globalThis.document={querySelector:node};
let response={level:'undergrad',label:'本科',items:[],fetched:0,full:false},calls=[];
const ui=createCampusUI({getState:createState,commit:async()=>{},toast:()=>{},confirm:async()=>true,render:()=>{},api:async(path)=>{calls.push(path);if(path.startsWith('/api/scores'))return response;return {saved:true,store_desc:'测试加密存储',message:'可访问'}}});
let count=0;
const check=async(name,fn)=>{await fn();count++;console.log('PASS',name)};
await check('explicit empty results render without a crash',async()=>{
 await ui.click('campus-online-score',{});
 assert.match(node('#online-score').innerHTML,/本次查询返回空列表/);
 assert.match(node('#online-score').innerHTML,/总数未确认/);
});
await check('null result rows show a format error rather than no grades',async()=>{
 response={...response,items:null};await ui.click('campus-online-score',{});
 assert.match(node('#online-score').innerHTML,/格式异常/);
 assert.doesNotMatch(node('#online-score').innerHTML,/返回空列表/);
});
await check('zero credit and GPA survive rendering; untrusted titles escape',async()=>{
 response={label:'本科',items:[{name:'<img src=x>',credit:0,score:'F',gpa:0},{name:'missing'}],fetched:2,total:2,full:true};
 await ui.click('campus-online-score',{});const html=node('#online-score').innerHTML;
 assert.match(html,/&lt;img src=x&gt;/);assert.doesNotMatch(html,/<img src=x>/);
 assert.match(html,/<td>0<\/td><td>F<\/td><td>0<\/td>/);assert.match(html,/未提供/);
});
await check('partial metadata stays visibly partial',async()=>{
 response={...response,total:80,full:false,note:'仅显示第一页，尚未取全'};await ui.click('campus-online-score',{});
 assert.match(node('#online-score').innerHTML,/总计 80 条记录/);assert.match(node('#online-score').innerHTML,/尚未取全/);
});
await check('changing academic level clears old grades and probes selected business',async()=>{
 await ui.change({target:{id:'online-score-level',value:'graduate'}});
 assert.doesNotMatch(node('#online-score').innerHTML,/&lt;img/);
 await ui.click('campus-session-check',{});assert.equal(calls.at(-1),'/api/session/check?level=graduate');
 await ui.click('campus-online-score',{});assert.equal(calls.at(-1),'/api/scores?level=graduate');
 assert.match(ui.grades(),/value="graduate" selected/);
});
await check('saving or clearing a session removes previous personal results',async()=>{
 node('#session-cookie').value='test-only';await ui.click('campus-session-save',{});
 assert.equal(node('#session-cookie').value,'');assert.doesNotMatch(node('#online-score').innerHTML,/&lt;img/);
 await ui.click('campus-online-score',{});node('#session-cookie').value='unsaved-test-cookie';await ui.click('campus-session-clear',{});
 assert.equal(node('#session-cookie').value,'');
 assert.doesNotMatch(node('#online-score').innerHTML,/&lt;img/);
 assert.match(ui.grades(),/不会注销浏览器或撤销学校会话/);
});
console.log(`${count} session UI checks passed`);

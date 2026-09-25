import assert from 'node:assert/strict';
import {pianoRoomsHTML,pianoMyHTML,pianoLoginHTML,PIANO_SITE} from './assets/garden/piano.mjs';
let checks=0;function test(name,fn){fn();checks++;console.log('PASS',name)}

test('琴房列表转义学校返回的 HTML，缺失字段显示破折号',()=>{
 const html=pianoRoomsHTML([{name:'<img src=x onerror=alert(1)>301',device:'',manager:'张',desc:'有钢琴'}]);
 assert.ok(html.includes('&lt;img'), '标签必须被转义');
 assert.ok(!html.includes('<img src=x'), '不能留下原始标签');
 assert.ok(html.includes('—'), '空字段应显示破折号');
 assert.ok(html.includes('有钢琴'));
});
test('琴房列表为空时如实提示，不显示假数据',()=>{
 const html=pianoRoomsHTML([]);
 assert.ok(html.includes('没有拿到琴房列表'), '空列表要有如实提示');
 assert.ok(!html.includes('<tr>'), '空列表不应有数据行');
});
test('我的预约映射 琴房/时间段/状态 并转义',()=>{
 const html=pianoMyHTML([{room:'<b>301</b>',time:'10:00-11:00',sign:'已签到'}]);
 assert.ok(html.includes('&lt;b&gt;301&lt;/b&gt;'), '预约琴房名要转义');
 assert.ok(html.includes('10:00-11:00')&&html.includes('已签到'));
 assert.ok(pianoMyHTML([]).includes('暂时没有你的预约记录'));
});
test('登录表单不自动填充密码，且给出「不保存凭据」的说明',()=>{
 const html=pianoLoginHTML();
 assert.ok(html.includes('id="piano-card"')&&html.includes('id="piano-pwd"'));
 assert.ok(html.includes('type="password"')&&html.includes('autocomplete="off"'),'密码框不应自动填充');
 assert.ok(html.includes('不会保存'),'要明示凭据不保存');
});
test('跳转原系统的地址指向前端页面而非接口',()=>{
 assert.match(PIANO_SITE,/:8080\//);
 assert.ok(!PIANO_SITE.includes(':60837'),'跳转应是给人看的页面，不是接口端口');
});

console.log(checks+' piano checks passed');

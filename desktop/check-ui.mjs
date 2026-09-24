import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createState,act,settle,normalize,gpa,activePet,petSprite} from './assets/garden/engine.mjs';
let checks=0;function test(name,fn){fn();checks++;console.log('PASS',name)}
const now=new Date('2026-09-18T10:00:00').getTime();
test('one harvest cannot be claimed twice; inventory can be sold',()=>{let s=createState(now);assert.throws(()=>act(s,{type:'harvest',index:0},now));s=act(s,{type:'water',index:0},now+1000);const due=s.game.plots[0].ready;assert.equal(due,now+45250);assert.throws(()=>act(s,{type:'water',index:0},now+2000));s=act(s,{type:'harvest',index:0},due);assert.equal(s.game.stock.radish,2);assert.throws(()=>act(s,{type:'harvest',index:0},due));s=act(s,{type:'sell',crop:'radish'},due);assert.equal(s.game.coins,52);assert.equal(s.game.stock.radish,0)});
test('seed spending, locked plots and level requirements',()=>{let s=createState(now);const before=structuredClone(s);assert.throws(()=>act(s,{type:'plant',index:3,crop:'radish'},now));assert.deepEqual(s,before);s=act(s,{type:'plant',index:1,crop:'radish'},now);assert.equal(s.game.seeds.radish,3);assert.throws(()=>act(s,{type:'plant',index:1,crop:'radish'},now));assert.throws(()=>act(s,{type:'buySeed',crop:'lychee'},now));s=act(s,{type:'gift'},now);s=act(s,{type:'unlock',index:3},now);assert.equal(s.game.coins,0);assert.equal(s.game.plots[3],null)});
test('daily gifts and quest rewards are one-time, clock rollback cannot reset',()=>{let s=act(createState(now),{type:'gift'},now);assert.throws(()=>act(s,{type:'gift'},now));s=act(s,{type:'plant',index:1,crop:'radish'},now);s=act(s,{type:'quest',id:'plant'},now);assert.throws(()=>act(s,{type:'quest',id:'plant'},now));assert.throws(()=>act(s,{type:'gift'},now-86400000));s=act(s,{type:'gift'},now+86400000);assert.equal(s.game.daily.gift,true)});
test('focus survives reload, early/duplicate claims fail',()=>{let s=act(createState(now),{type:'focusStart',minutes:5},now);s=normalize(JSON.parse(JSON.stringify(s)),now+120000);assert.throws(()=>act(s,{type:'focusClaim'},now+120000));s=act(s,{type:'focusClaim'},now+300000);assert.equal(s.game.stats.minutes,5);assert.equal(s.game.coins,45);assert.throws(()=>act(s,{type:'focusClaim'},now+300000))});
test('offline pet does not die and sleep restores energy',()=>{let s=createState(now);activePet(s.game).energy=20;s=act(s,{type:'sleep'},now);s=settle(s,now+3600000);assert.equal(activePet(s.game).energy,50);s=settle(s,now+86400000*10);assert.ok(activePet(s.game).hunger>=15);assert.ok(activePet(s.game).mood>=20);assert.equal(s.game.plots[0].crop,'radish')});
// 多伙伴：新存档默认荔宝且一开局就有两只（否则切换器对谁都不显示）；
// 切到另一位时成长各自独立，不继承也不清零；旧单伙伴存档迁移后保留用户起的名字。
test('pets: libao default, independent growth, old save migrates with its name',()=>{
 let s=createState(now);
 assert.equal(activePet(s.game).name,'荔宝');
 assert.equal(petSprite(activePet(s.game)),'libao');
 assert.equal(s.game.pets.length,2,'新存档一开局就要有两只，切换器才对用户可见');
 s=act(s,{type:'pat'},now);
 assert.ok(activePet(s.game).say.length>0,'互动后宠物要说话');
 s=act(s,{type:'switchPet',index:1},now);
 assert.equal(activePet(s.game).species,'chestnut');
 assert.equal(activePet(s.game).xp,0,'切到栗栗时它的成长是独立的，不该继承荔宝的');
 const old=JSON.parse(JSON.stringify(s));old.schema=2;old.game.pet=old.game.pets[0];delete old.game.pets;delete old.game.active;
 const m=normalize(old,now);
 assert.equal(m.schema,3);assert.equal(m.game.pets.length,1);assert.equal(activePet(m.game).species,'chestnut');
});
test('todos reward completion only once',()=>{let s=act(createState(now),{type:'todoAdd',id:'task',text:'test'},now);for(let i=0;i<3;i++)s=act(s,{type:'todoToggle',id:'task'},now);assert.equal(activePet(s.game).xp,2);assert.equal(s.game.stats.tasks,1)});
test('invalid imports are rejected; unknown root fields are excluded',()=>{assert.throws(()=>normalize({schema:1},now));let s=createState(now);s.game.coins=-1;assert.throws(()=>normalize(s,now));s=createState(now);s.game.plots[0].crop='unknown';assert.throws(()=>normalize(s,now));s=createState(now);s.password='must-not-export';activePet(s.game).password='must-not-export';s.semester='2026-09-01';s=normalize(s,now);assert.equal(s.password,undefined);assert.equal(activePet(s.game).password,undefined);assert.equal(s.semester,'2026-09-01')});
test('GPA is credit-weighted without score conversion',()=>assert.deepEqual(gpa([{credit:3,point:4},{credit:1,point:2}]),{credits:4,value:3.5}));
test('independent decoration ownership and achievement reward',()=>{let s=createState(now);s=act(s,{type:'decor',id:'flower'},now);assert.equal(s.game.coins,5);s=act(s,{type:'decor',id:'flower'},now);assert.equal(s.game.coins,5);assert.equal(s.game.equipped.length,0);s=act(s,{type:'harvest',index:0},now+60000);s=act(s,{type:'achievement',id:'harvest'},now+60000);assert.equal(s.game.coins,35);assert.throws(()=>act(s,{type:'achievement',id:'harvest'},now+60000))});
const html=readFileSync(new URL('./index.html',import.meta.url),'utf8');test('HTML IDs are unique',()=>{const ids=[...html.matchAll(/\bid="([^"]+)"/g)].map(m=>m[1]);assert.equal(new Set(ids).size,ids.length)});
test('version comes from the API, never hardcoded in page sources',()=>{assert.ok(!/beta\d/.test(html),'index.html 里又写死了版本号，改 internal/version/VERSION 才会全项目一致');assert.ok(html.includes('id="app-badge"'),'顶栏缺少版本号挂载点，页面会一直显示不出当前版本');for(const name of ['app.mjs','engine.mjs','campus-ui.mjs','campus.mjs','school.mjs','booking.mjs','academic.mjs','notices.mjs','network-status.mjs','pixel.mjs']){const src=readFileSync(new URL('./assets/garden/'+name,import.meta.url),'utf8');assert.ok(!/beta\d/.test(src),name+' 里写死了版本号')}});
test('unverified features share one label',()=>{const label=readFileSync(new URL('./assets/garden/labels.mjs',import.meta.url),'utf8');assert.match(label,/接入测试 · 未经真实验收/,'统一措辞被改了，全项目的验收状态提示会跟着漂');for(const name of ['app.mjs','engine.mjs','campus-ui.mjs','campus.mjs','school.mjs','booking.mjs','academic.mjs','notices.mjs','network-status.mjs']){const src=readFileSync(new URL('./assets/garden/'+name,import.meta.url),'utf8');assert.ok(!/接入测试|实验功能/.test(src),name+' 自己写了一份验收状态措辞，应该用 labels.mjs 的 unverifiedBadge()')}for(const name of ['school.mjs','campus-ui.mjs']){const src=readFileSync(new URL('./assets/garden/'+name,import.meta.url),'utf8');assert.match(src,/unverifiedBadge\(\)/,name+' 该标未验收的功能没有用统一标记')}});
console.log(`${checks} checks passed`);

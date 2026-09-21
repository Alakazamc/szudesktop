// Original local-only garden rules. No accounts, passwords, or remote services.
export const CROPS={
 radish:{name:'小萝卜',icon:'radish',time:60000,price:4,sell:6,yield:2,level:1},
 strawberry:{name:'草莓',icon:'strawberry',time:300000,price:12,sell:12,yield:2,level:1},
 blueberry:{name:'蓝莓',icon:'blueberry',time:1200000,price:24,sell:23,yield:2,level:2},
 lychee:{name:'荔枝',icon:'lychee',time:3600000,price:40,sell:38,yield:3,level:3},
};
export const DECOR={flower:{name:'窗边小花',price:35},scarf:{name:'猫咪围巾',price:60},lantern:{name:'暖光灯笼',price:90}};
export const QUESTS={care:{name:'陪伴伙伴 3 次',target:3,reward:15},plant:{name:'种下一颗种子',target:1,reward:10},harvest:{name:'收获一块农田',target:1,reward:15},focus:{name:'完成一次专注',target:1,reward:25}};
export const dayKey=t=>{const d=new Date(t);return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`};
export const level=g=>Math.min(20,1+Math.floor(g.pet.xp/50));
export function createState(now=Date.now()){
 return {schema:2,profile:{name:'',college:''},preferences:{theme:'day',motion:true,onboarded:false},todos:[],courses:[],reminders:[],semester:'',
 game:{created:now,last:now,coins:40,food:3,seeds:{radish:4,strawberry:2,blueberry:0,lychee:0},stock:{radish:0,strawberry:0,blueberry:0,lychee:0},
 plots:[{crop:'radish',planted:now,ready:now+60000,watered:false},null,null,'locked','locked','locked'],pet:{name:'栗栗',xp:0,bond:10,hunger:80,energy:85,mood:85,sleeping:false,lastPat:0,lastPlay:0},
 daily:{day:dayKey(now),gift:false,care:0,plant:0,harvest:0,focus:0,claimed:[]},stats:{harvest:0,focus:0,minutes:0,planted:1,tasks:0},discovered:[],decor:[],equipped:[],achievements:[],focus:null,log:[{time:now,text:'欢迎来到荔枝庭院。第一块萝卜地已经种好，记得来收获。'}]}};
}
const clamp=(n,min,max)=>Math.max(min,Math.min(max,n));
function check(ok,message){if(!ok)throw Error(message)}
function note(g,text,now){g.log.unshift({time:now,text});g.log=g.log.slice(0,30)}
export function normalize(input,now=Date.now()){
 check(input&&input.schema===2&&input.game&&Array.isArray(input.game.plots),'不支持的存档格式，请选择本应用导出的存档');
 const s=structuredClone(input),g=s.game;
 for(const key of ['coins','food']){check(Number.isFinite(g[key])&&g[key]>=0&&g[key]<=1e8,'存档资源格式错误')}
 check(g.pet&&Number.isFinite(g.pet.xp)&&g.pet.xp>=0&&g.pet.xp<=1e6,'伙伴存档格式错误');
 for(const k of ['hunger','energy','mood','bond']){check(Number.isFinite(g.pet[k]),'伙伴状态格式错误');g.pet[k]=clamp(g.pet[k],0,100)}
 check(Number.isFinite(g.last)&&g.last>0&&g.last<=now+86400000,'存档时间异常');
 check(g.plots.length===6,'农田存档格式错误');
 for(const p of g.plots)if(p&&p!=='locked')check(CROPS[p.crop]&&Number.isFinite(p.planted)&&Number.isFinite(p.ready),'作物存档格式错误');
 for(const k of Object.keys(CROPS))for(const bag of ['seeds','stock'])check(g[bag]&&Number.isInteger(g[bag][k])&&g[bag][k]>=0&&g[bag][k]<=1e7,'背包格式错误');
 check(g.daily&&typeof g.daily.day==='string'&&Array.isArray(g.daily.claimed),'每日任务格式错误');
 for(const k of Object.keys(QUESTS))check(Number.isFinite(g.daily[k])&&g.daily[k]>=0,'每日任务数值错误');
 for(const k of ['harvest','focus','minutes','planted','tasks'])check(g.stats&&Number.isFinite(g.stats[k])&&g.stats[k]>=0,'累计记录格式错误');
 for(const k of ['discovered','decor','equipped','achievements','log'])check(Array.isArray(g[k]),'收藏存档格式错误');
 g.decor=g.decor.filter(k=>DECOR[k]);g.equipped=g.equipped.filter(k=>g.decor.includes(k));g.discovered=g.discovered.filter(k=>CROPS[k]);
 g.log=g.log.slice(0,30).filter(x=>x&&typeof x.text==='string'&&Number.isFinite(x.time)).map(x=>({time:x.time,text:x.text.slice(0,180)}));
 check(!g.focus||(Number.isFinite(g.focus.end)&&Number.isFinite(g.focus.duration)&&g.focus.duration>=1&&g.focus.duration<=120),'专注存档格式错误');
 g.pet.name=String(g.pet.name||'栗栗').slice(0,12);
 for(const k of ['lastPat','lastPlay'])check(Number.isFinite(g.pet[k])&&g.pet[k]>=0,'互动时间格式错误');g.pet.sleeping=!!g.pet.sleeping;
 s.profile={name:String(s.profile?.name||'').slice(0,20),college:String(s.profile?.college||'').slice(0,40)};
 s.preferences={theme:s.preferences?.theme==='night'?'night':'day',motion:s.preferences?.motion!==false,onboarded:s.preferences?.onboarded===true};
 s.todos=(Array.isArray(s.todos)?s.todos:[]).slice(0,100).filter(x=>x&&typeof x.id==='string'&&typeof x.text==='string').map(x=>({id:x.id.slice(0,60),text:x.text.slice(0,120),done:!!x.done,rewarded:!!x.rewarded}));
 s.courses=(Array.isArray(s.courses)?s.courses:[]).slice(0,300).filter(x=>x&&Number.isFinite(x.credit)&&Number.isFinite(x.point)&&x.credit>0&&x.credit<=100&&x.point>=0&&x.point<=5).map(x=>({name:String(x.name||'课程').slice(0,100),credit:x.credit,point:x.point,term:String(x.term||'').slice(0,40),code:String(x.code||'').slice(0,40),level:['undergrad','graduate'].includes(x.level)?x.level:'',grade:String(x.grade||'').slice(0,20),source:String(x.source||'手动录入').slice(0,30),included:x.included!==false}));
 s.reminders=(Array.isArray(s.reminders)?s.reminders:[]).filter(x=>x&&typeof x.id==='string'&&typeof x.place==='string'&&Number.isFinite(x.start)&&Number.isFinite(x.end)&&x.end>x.start&&x.end-x.start<=86400000).slice(0,50).map(x=>({id:x.id.slice(0,80),place:x.place.slice(0,80),start:x.start,end:x.end}));
 s.semester=/^\d{4}-\d{2}-\d{2}$/.test(s.semester||'')?s.semester:'';
 const clean={schema:2,profile:s.profile,preferences:s.preferences,todos:s.todos,courses:s.courses,reminders:s.reminders,semester:s.semester,game:{}};
 for(const k of Object.keys(createState(now).game))clean.game[k]=g[k];
 clean.game.pet=Object.fromEntries(Object.keys(createState(now).game.pet).map(k=>[k,g.pet[k]]));
 return settle(clean,now);
}
export function settle(state,now=Date.now()){
 const s=structuredClone(state),g=s.game;now=Math.max(now,g.last);
 const hours=clamp((now-g.last)/3600000,0,48);
 g.pet.hunger=clamp(g.pet.hunger-hours*2,15,100);g.pet.mood=clamp(g.pet.mood-hours,20,100);
 g.pet.energy=clamp(g.pet.energy+hours*(g.pet.sleeping?30:-1),15,100);
 g.last=now;
 const day=dayKey(now);
 if(day>g.daily.day)g.daily={day,gift:false,care:0,plant:0,harvest:0,focus:0,claimed:[]};
 return s;
}
export function achievementList(g){return [
 {id:'harvest',name:'第一篮收成',hint:'收获一次',done:g.stats.harvest>=1},
 {id:'friend',name:'熟悉的朋友',hint:'伙伴达到 3 级',done:level(g)>=3},
 {id:'focus',name:'专注的午后',hint:'完成 3 次专注',done:g.stats.focus>=3},
 {id:'gardener',name:'小小园艺家',hint:'累计收获 10 次',done:g.stats.harvest>=10},
 {id:'land',name:'庭院主人',hint:'解锁全部 6 块地',done:g.plots.every(p=>p!=='locked')},
 {id:'collection',name:'四季收藏家',hint:'收获全部 4 种作物',done:g.discovered.length===4},
]}
export function act(state,a,now=Date.now()){
 const s=settle(state,now),g=s.game,p=g.pet;now=g.last;
 const care=()=>{g.daily.care++;p.bond=clamp(p.bond+3,0,100)};
 switch(a.type){
 case 'gift':check(!g.daily.gift,'今天的补给已经领过啦');g.daily.gift=true;g.coins+=20;g.food++;g.seeds.radish+=2;note(g,'领取每日补给：20 荔枝币、1 份食物、2 颗萝卜种子。',now);break;
 case 'pat':check(now-p.lastPat>=10000,'让它享受一下，稍等 10 秒再摸摸');p.lastPat=now;p.xp+=2;p.mood=clamp(p.mood+5,0,100);care();note(g,'摸摸头，'+p.name+'舒服地眯起了眼。',now);break;
 case 'feed':check(!p.sleeping,'先唤醒伙伴再喂食');check(p.hunger<98,'它已经吃饱了，留着下次吧');
  if(a.crop){check(CROPS[a.crop]&&g.stock[a.crop]>0,'背包里没有这种作物');g.stock[a.crop]--;}else{check(g.food>0,'食物用完了，可以去集市购买');g.food--;}
  p.hunger=clamp(p.hunger+30,0,100);p.energy=clamp(p.energy+5,0,100);p.xp+=8;care();note(g,'吃饱啦，伙伴的亲密度和成长增加了。',now);break;
 case 'play':check(!p.sleeping,'先唤醒伙伴');check(p.energy>=25,'它有点累了，让它休息一会儿');check(now-p.lastPlay>=30000,'再等一会儿，30 秒后可以继续玩');p.lastPlay=now;p.energy-=10;p.mood=clamp(p.mood+20,0,100);p.xp+=5;care();note(g,'陪'+p.name+'玩了一会儿，心情变好了。',now);break;
 case 'sleep':p.sleeping=!p.sleeping;note(g,p.sleeping?p.name+'睡下了，休息时会恢复精力。':p.name+'醒来，伸了一个大懒腰。',now);break;
 case 'rename':check(String(a.name||'').trim().length>0,'给伙伴取个名字吧');p.name=String(a.name).trim().slice(0,12);break;
 case 'plant':{
  const c=CROPS[a.crop];check(Number.isInteger(a.index)&&a.index>=0&&a.index<6&&g.plots[a.index]===null,'请选择空地');check(c&&level(g)>=c.level,'伙伴等级还不够');check(g.seeds[a.crop]>0,'这种种子用完了，去集市补充吧');g.seeds[a.crop]--;g.plots[a.index]={crop:a.crop,planted:now,ready:now+c.time,watered:false};g.stats.planted++;g.daily.plant++;note(g,'种下了'+c.name+'，离线时也会继续生长。',now);break;}
 case 'water':{const x=g.plots[a.index];check(x&&x!=='locked','这块地还没有作物');check(!x.watered,'已经浇过水了');check(now<x.ready,'已经成熟，可以收获了');x.ready=now+Math.ceil((x.ready-now)*0.75);x.watered=true;p.xp+=1;note(g,'浇水完成，剩余生长时间缩短四分之一。',now);break;}
 case 'harvest':{const x=g.plots[a.index];check(x&&x!=='locked'&&now>=x.ready,'作物还没有成熟');const c=CROPS[x.crop];g.stock[x.crop]+=c.yield;if(!g.discovered.includes(x.crop))g.discovered.push(x.crop);g.plots[a.index]=null;g.stats.harvest++;g.daily.harvest++;p.xp+=10;note(g,'收获 '+c.name+' ×'+c.yield+'，已放入背包。',now);break;}
 case 'unlock':{const n=g.plots.filter(x=>x!=='locked').length;const cost=60+(n-3)*30;check(g.plots[a.index]==='locked','这块地已经解锁');check(g.coins>=cost,'荔枝币不够，收获后去集市出售作物吧');g.coins-=cost;g.plots[a.index]=null;note(g,'新开垦了一块农田。',now);break;}
 case 'buySeed':{const c=CROPS[a.crop];check(c&&level(g)>=c.level,'需要更高的伙伴等级');check(g.coins>=c.price,'荔枝币不够');g.coins-=c.price;g.seeds[a.crop]++;break;}
 case 'sell':{const c=CROPS[a.crop],n=g.stock[a.crop]||0;check(c&&n>0,'没有可出售的作物');g.coins+=n*c.sell;g.stock[a.crop]=0;note(g,'出售'+c.name+' ×'+n+'，获得 '+(n*c.sell)+' 荔枝币。',now);break;}
 case 'buyFood':check(g.coins>=8,'需要 8 荔枝币');g.coins-=8;g.food++;break;
 case 'decor':{const d=DECOR[a.id];check(d,'没有这件装饰');if(!g.decor.includes(a.id)){check(g.coins>=d.price,'荔枝币不够');g.coins-=d.price;g.decor.push(a.id);}g.equipped=g.equipped.includes(a.id)?g.equipped.filter(x=>x!==a.id):[...g.equipped,a.id];break;}
 case 'quest':{const q=QUESTS[a.id];check(q&&g.daily[a.id]>=q.target,'目标还没有完成');check(!g.daily.claimed.includes(a.id),'奖励已领取');g.daily.claimed.push(a.id);g.coins+=q.reward;note(g,'完成每日目标：'+q.name+'。',now);break;}
 case 'achievement':{const x=achievementList(g).find(x=>x.id===a.id);check(x?.done,'成就还没有完成');check(!g.achievements.includes(a.id),'奖励已领取');g.achievements.push(a.id);g.coins+=30;note(g,'获得纪念章：'+x.name+'。',now);break;}
 case 'focusStart':check(!g.focus,'请先完成或取消当前专注');check([5,25,45].includes(a.minutes),'请选择 5、25 或 45 分钟');g.focus={end:now+a.minutes*60000,duration:a.minutes};break;
 case 'focusCancel':g.focus=null;break;
 case 'focusClaim':check(g.focus&&now>=g.focus.end,'专注还没结束');g.stats.minutes+=g.focus.duration;g.stats.focus++;g.daily.focus++;g.coins+=g.focus.duration;p.xp+=10;p.mood=clamp(p.mood+10,0,100);note(g,'完成 '+g.focus.duration+' 分钟专注，获得等量荔枝币。',now);g.focus=null;break;
 case 'todoAdd':{const text=String(a.text||'').trim();check(text,'先写下一件小事');check(s.todos.length<100,'清单已满，请先清理已完成事项');s.todos.push({id:String(a.id),text:text.slice(0,120),done:false,rewarded:false});break;}
 case 'todoToggle':{const t=s.todos.find(x=>x.id===a.id);check(t,'没有找到事项');t.done=!t.done;if(t.done&&!t.rewarded){t.rewarded=true;g.stats.tasks++;p.xp+=2;}break;}
 case 'todoDelete':s.todos=s.todos.filter(x=>x.id!==a.id);break;
 default:throw Error('未知操作');
 }
 return s;
}
export function gpa(courses){courses=courses.filter(c=>c.included!==false);const total=courses.reduce((n,c)=>n+c.credit,0);return {credits:total,value:total?courses.reduce((n,c)=>n+c.credit*c.point,0)/total:0}}

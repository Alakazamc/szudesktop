import assert from 'node:assert/strict';
import {petWindowOptions,petWindowBounds,petScaleClamp,petSay,petSpriteFor,activePetOf,isPetSender,PET_WIDTH,PET_HEIGHT,PET_MARGIN,PET_SAY_MAX,PET_SCALE_MIN,PET_SCALE_MAX,PET_SCALE_DEFAULT,PET_SCALE_PRESETS,PET_ACTIONS,petBaseAction,petActionFor,petIdleWeights,petIdleAction,petPresetFor} from './pet-policy.mjs';

// 窗口选项：spec §7 的每一项都要钉死。
const wa={x:0,y:0,width:1920,height:1080};
const o=petWindowOptions(wa);
assert.equal(o.frame,false);
assert.equal(o.transparent,true);
assert.equal(o.alwaysOnTop,true);
assert.equal(o.skipTaskbar,true);
assert.equal(o.resizable,false);
assert.equal(o.focusable,false);
assert.equal(o.hasShadow,false);
assert.equal(o.width,PET_WIDTH);
assert.equal(o.height,PET_HEIGHT);
// 贴主工作区右下角（留 PET_MARGIN 边距）。
assert.equal(o.x,wa.x+wa.width-PET_WIDTH-PET_MARGIN);
assert.equal(o.y,wa.y+wa.height-PET_HEIGHT-PET_MARGIN);
// 副屏 workArea 的原点非 0 时也要算对。
const wa2={x:1920,y:-300,width:1280,height:1024};
const o2=petWindowOptions(wa2);
assert.equal(o2.x,1920+1280-PET_WIDTH-PET_MARGIN);
assert.equal(o2.y,-300+1024-PET_HEIGHT-PET_MARGIN);

// 台词：60 字上限，镜像 engine.mjs 的 say()。
assert.equal(petSay('嗨，我是荔宝！'),'嗨，我是荔宝！');
assert.equal(petSay('a'.repeat(PET_SAY_MAX)).length,PET_SAY_MAX);
assert.equal(petSay('a'.repeat(PET_SAY_MAX)).length,60);
assert.equal(petSay('荔'.repeat(61)),'荔'.repeat(60));
assert.equal(petSay('b'.repeat(120)).length,60);
assert.equal(petSay(2026),'2026');

// 立绘映射：默认荔宝；栗栗按 sleeping/mood 四帧。
assert.equal(petSpriteFor({species:'libao',mood:5,sleeping:true}),'libao');
assert.equal(petSpriteFor({species:'chestnut',sleeping:true,mood:90}),'cat-sleep');
assert.equal(petSpriteFor({species:'chestnut',sleeping:false,mood:34}),'cat-sad');
assert.equal(petSpriteFor({species:'chestnut',sleeping:false,mood:35}),'cat-normal');
assert.equal(petSpriteFor({species:'chestnut',sleeping:false,mood:50}),'cat-normal');
assert.equal(petSpriteFor({species:'chestnut',sleeping:false,mood:65}),'cat-normal');
assert.equal(petSpriteFor({species:'chestnut',sleeping:false,mood:66}),'cat-happy');
// 未知/缺失种类回退默认荔宝（与 engine 的 PETS[DEFAULT_PET] 兜底一致）。
assert.equal(petSpriteFor({species:'unknown',sleeping:false,mood:90}),'libao');
assert.equal(petSpriteFor({}),'libao');

// activePet：镜像 engine.mjs 的 activePet()，读不到返回 null 不伪造。
const pets=[{species:'libao'},{species:'chestnut',mood:90,sleeping:false}];
assert.equal(activePetOf({pets,active:1}),pets[1]);
assert.equal(activePetOf({pets,active:9}),pets[0]);
assert.equal(activePetOf({pets:[]}),null);
assert.equal(activePetOf(null),null);
assert.equal(activePetOf({}),null);

// 可信来源：结构与 check-window-policy.mjs 的 isTrustedSender 测试一致。
const petUrl='file:///D:/szudesktop/desktop/electron/pet.html';
const frame={url:petUrl},wc={mainFrame:frame},win={webContents:wc};
assert.equal(isPetSender({sender:wc,senderFrame:frame},win,petUrl),true);
assert.equal(isPetSender({sender:wc,senderFrame:{url:petUrl}},win,petUrl),false);
assert.equal(isPetSender({sender:{},senderFrame:frame},win,petUrl),false);
assert.equal(isPetSender({sender:wc,senderFrame:frame},null,petUrl),false);
frame.url='https://szu.edu.cn/';
assert.equal(isPetSender({sender:wc,senderFrame:frame},win,petUrl),false);
frame.url='file:///D:/evil/pet.html';
assert.equal(isPetSender({sender:wc,senderFrame:frame},win,petUrl),false);

// 缩放：归一化。越界夹紧、非有限值回落默认、保留 2 位小数。
assert.equal(petScaleClamp(0.4),0.4);
assert.equal(petScaleClamp(2),2);
assert.equal(petScaleClamp(0.1),PET_SCALE_MIN);
assert.equal(petScaleClamp(9),PET_SCALE_MAX);
assert.equal(petScaleClamp(1.23456),1.23);
assert.equal(petScaleClamp('1.5'),1.5);
assert.equal(petScaleClamp(NaN),PET_SCALE_DEFAULT);
// ±Infinity 也不是有限数，和 NaN 一样回落默认值——不会悄悄变成 2.0。
assert.equal(petScaleClamp(Infinity),PET_SCALE_DEFAULT);
assert.equal(petScaleClamp(-Infinity),PET_SCALE_DEFAULT);
assert.equal(petScaleClamp(undefined),PET_SCALE_DEFAULT);
// Number(null)===0 是有限数，会被夹到下界而不是回落默认值——这是有意行为，钉死它。
assert.equal(petScaleClamp(null),PET_SCALE_MIN);
assert.equal(petScaleClamp({}),PET_SCALE_DEFAULT);

// 预设档位：三个、id 唯一、scale 都在合法区间内。
assert.deepEqual(PET_SCALE_PRESETS.map(p=>p.id),['small','medium','large']);
assert.equal(new Set(PET_SCALE_PRESETS.map(p=>p.id)).size,3);
for(const p of PET_SCALE_PRESETS){
  assert.ok(p.scale>=PET_SCALE_MIN&&p.scale<=PET_SCALE_MAX,p.id);
  assert.equal(petScaleClamp(p.scale),p.scale,p.id);
}

// 窗口几何：基准 × scale，贴 workArea 右下角。
const b1=petWindowBounds(wa,1);
assert.deepEqual(b1,{x:1636,y:736,width:260,height:320});
const bSmall=petWindowBounds(wa,0.4);
assert.deepEqual(bSmall,{x:1792,y:928,width:104,height:128});
const bLarge=petWindowBounds(wa,2);
assert.deepEqual(bLarge,{x:1376,y:416,width:520,height:640});
// 副屏 workArea 原点非 0。
assert.deepEqual(petWindowBounds(wa2,1),{x:2916,y:380,width:260,height:320});
// 右下角锚定：scale 变化时右边缘与下边缘不动。
for(const s of [0.4,0.6,1,1.5,2]){
  const b=petWindowBounds(wa,s);
  assert.equal(b.x+b.width,wa.width-PET_MARGIN,`scale ${s} 右边缘`);
  assert.equal(b.y+b.height,wa.height-PET_MARGIN,`scale ${s} 下边缘`);
}
// 极端 scale 也不产生 0 或负尺寸。
for(const s of [PET_SCALE_MIN,PET_SCALE_MAX]){
  const b=petWindowBounds(wa,s);
  assert.ok(b.width>0&&b.height>0);
}

// 向后兼容：不传 scale 必须与今天的输出逐字段相同。
const legacy=petWindowOptions(wa);
assert.equal(legacy.width,PET_WIDTH);
assert.equal(legacy.height,PET_HEIGHT);
assert.equal(legacy.x,wa.x+wa.width-PET_WIDTH-PET_MARGIN);
assert.equal(legacy.y,wa.y+wa.height-PET_HEIGHT-PET_MARGIN);
assert.equal(legacy.frame,false);
assert.equal(legacy.resizable,false);
// 传 scale 时窗口尺寸随之变化，安全项不变。
const scaled=petWindowOptions(wa,2);
assert.equal(scaled.width,520);
assert.equal(scaled.height,640);
assert.equal(scaled.x,1376);
assert.equal(scaled.y,416);
assert.equal(scaled.resizable,false);
assert.equal(scaled.focusable,false);
assert.equal(scaled.transparent,true);
// workArea 缺字段时不抛异常。
const partial=petWindowBounds({width:1920,height:1080},1);
assert.deepEqual(partial,{x:1636,y:736,width:260,height:320});

// 拖动后的位置恢复；放大或断开副屏后，整个窗口仍留在目标工作区。
assert.deepEqual(petWindowBounds(wa,1,{x:200,y:300}),{x:200,y:300,width:260,height:320});
assert.deepEqual(petWindowBounds(wa,2,{x:1700,y:900}),{x:1400,y:440,width:520,height:640});
assert.deepEqual(petWindowBounds(wa,1,{x:-1200,y:-900}),{x:0,y:0,width:260,height:320});
assert.deepEqual(petWindowBounds(wa2,1,{x:2100.4,y:-299.6}),{x:2100,y:-300,width:260,height:320});
assert.equal(petWindowOptions(wa,1,{x:100,y:200}).x,100);
assert.equal(petWindowOptions(wa,1,{x:100,y:200}).y,200);
// 只接受完整的有限数坐标；旧配置和无效位置仍按默认停靠。
for(const position of [undefined,null,{}, {x:5}, {x:5,y:Infinity}, {x:'5',y:6}]){
  assert.deepEqual(petWindowBounds(wa,1,position),b1);
}
// 极小工作区按比例收缩有效尺寸，不能因 24px 留白或负坐标被挤出屏幕。
const tiny={x:-100,y:50,width:100,height:80};
assert.deepEqual(petWindowBounds(tiny,2,{x:999,y:-999}),{x:-65,y:50,width:65,height:80});
for(const area of [tiny,{x:20,y:-40,width:260,height:320},{width:1,height:1},{}]){
  const b=petWindowBounds(area,2,{x:-99999,y:99999});
  const x=area.x||0,y=area.y||0,width=area.width||1,height=area.height||1;
  assert.ok(b.x>=x&&b.y>=y,'窗口左上角在工作区内');
  assert.ok(b.x+b.width<=x+width&&b.y+b.height<=y+height,'窗口右下角在工作区内');
  assert.ok(b.width>=1&&b.height>=1,'窗口尺寸始终为正');
}

// 动作表：8 个，id 唯一，kind 只有 loop/once，duration 为正整数。
const ACTION_IDS=Object.keys(PET_ACTIONS);
assert.deepEqual(ACTION_IDS,['idle','happy','sad','sleep','blink','yawn','walk','react']);
assert.equal(new Set(ACTION_IDS).size,8);
for(const [id,spec] of Object.entries(PET_ACTIONS)){
  assert.ok(spec.kind==='loop'||spec.kind==='once',id);
  assert.ok(Number.isInteger(spec.duration)&&spec.duration>0,id);
  assert.equal(typeof spec.label,'string',id);
}
assert.equal(PET_ACTIONS.blink.kind,'once');
assert.equal(PET_ACTIONS.walk.kind,'once');
assert.equal(PET_ACTIONS.yawn.kind,'once');
assert.equal(PET_ACTIONS.react.kind,'once');
assert.equal(PET_ACTIONS.idle.kind,'loop');
assert.equal(PET_ACTIONS.happy.kind,'loop');
assert.equal(PET_ACTIONS.sad.kind,'loop');
assert.equal(PET_ACTIONS.sleep.kind,'loop');

// 基础动作：与 engine.mjs 的 petSprite() 同源判断。
assert.equal(petBaseAction(null),'idle');
assert.equal(petBaseAction({}),'idle');
assert.equal(petBaseAction({species:'chestnut',mood:90,sleeping:false}),'happy');
assert.equal(petBaseAction({species:'chestnut',mood:66,sleeping:false}),'happy');
assert.equal(petBaseAction({species:'chestnut',mood:65,sleeping:false}),'idle');
assert.equal(petBaseAction({species:'chestnut',mood:50,sleeping:false}),'idle');
assert.equal(petBaseAction({species:'chestnut',mood:35,sleeping:false}),'idle');
assert.equal(petBaseAction({species:'chestnut',mood:34,sleeping:false}),'sad');
assert.equal(petBaseAction({species:'chestnut',mood:1,sleeping:false}),'sad');
// 睡觉压过心情。
assert.equal(petBaseAction({species:'chestnut',mood:90,sleeping:true}),'sleep');
assert.equal(petBaseAction({species:'chestnut',mood:1,sleeping:true}),'sleep');
// mood 非数字时不当成 0，回落 idle。
assert.equal(petBaseAction({mood:'high',sleeping:false}),'idle');
assert.equal(petBaseAction({mood:NaN,sleeping:false}),'idle');

// 一次性覆盖优先；未知 id 与 null 被忽略。
assert.equal(petActionFor({mood:90,sleeping:false},'react'),'react');
assert.equal(petActionFor({mood:10,sleeping:false},'blink'),'blink');
assert.equal(petActionFor({mood:90,sleeping:true},'react'),'react');
assert.equal(petActionFor({mood:90,sleeping:false},'nope'),'happy');
assert.equal(petActionFor({mood:90,sleeping:false},null),'happy');
assert.equal(petActionFor({mood:90,sleeping:false},undefined),'happy');
assert.equal(petActionFor({mood:90,sleeping:false},42),'happy');
// 循环动作也可以被显式指定（ Phase 3 的 agent 事件可能直接推 sleep）。
assert.equal(petActionFor({mood:50,sleeping:false},'sleep'),'sleep');
assert.equal(petActionFor(null,null),'idle');

// 待机权重：睡觉不走动，低精力多打哈欠；全部为非负整数。
const awake=petIdleWeights({energy:80,sleeping:false});
const tired=petIdleWeights({energy:10,sleeping:false});
const asleep=petIdleWeights({energy:10,sleeping:true});
assert.equal(awake.walk,3);
assert.equal(asleep.walk,0);
assert.ok(tired.yawn>awake.yawn,'低精力时打哈欠权重必须更高');
for(const w of [awake,tired,asleep]){
  for(const v of Object.values(w)) assert.ok(Number.isInteger(v)&&v>=0);
}

// 待机挑选：确定性（同 tick 同输入同输出）。
for(let tick=0;tick<50;tick++){
  assert.equal(petIdleAction(tick,{energy:80,sleeping:false}),petIdleAction(tick,{energy:80,sleeping:false}),`tick ${tick} 必须可复现`);
}
// 返回值只可能是三个一次性动作或 null，永不 undefined。
const seen=new Set();
for(let tick=0;tick<2000;tick++){
  const pick=petIdleAction(tick,{energy:80,sleeping:false});
  assert.ok(pick===null||Object.hasOwn(PET_ACTIONS,pick),`tick ${tick} 返回了表外值 ${pick}`);
  assert.notEqual(pick,undefined);
  assert.notEqual(pick,'idle','待机挑选不应返回循环动作');
  seen.add(pick);
}
// 覆盖率：四个候选都出现过（否则加权是死的）。
assert.ok(seen.has('blink'),'2000 个 tick 内必须出现过眨眼');
assert.ok(seen.has('yawn'),'2000 个 tick 内必须出现过打哈欠');
assert.ok(seen.has('walk'),'2000 个 tick 内必须出现过走动');
assert.ok(seen.has(null),'2000 个 tick 内必须出现过「什么都不做」');
// 睡着时永远选不出走动。
for(let tick=0;tick<2000;tick++) assert.notEqual(petIdleAction(tick,{energy:10,sleeping:true}),'walk');

// 预设反查：命中档位返回 id，自定义值返回 null。
assert.equal(petPresetFor(0.6),'small');
assert.equal(petPresetFor(1),'medium');
assert.equal(petPresetFor(1.5),'large');
assert.equal(petPresetFor(1.23),null);
assert.equal(petPresetFor(0.4),null);
assert.equal(petPresetFor(NaN),null);

console.log('Pet policy: window options, say cap, sprite mapping and sender checks passed');

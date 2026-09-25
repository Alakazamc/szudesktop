import assert from 'node:assert/strict';
import {petWindowOptions,petSay,petSpriteFor,activePetOf,isPetSender,PET_WIDTH,PET_HEIGHT,PET_MARGIN,PET_SAY_MAX} from './pet-policy.mjs';

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

console.log('Pet policy: window options, say cap, sprite mapping and sender checks passed');

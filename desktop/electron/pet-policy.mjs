// 宠物窗的纯策略模块：不 import electron，便于像 window-policy.mjs 一样单测。
// 立绘与台词规则镜像 desktop/assets/garden/engine.mjs 的 petSprite()/say()。

// 窗口尺寸与右下角停靠留白（像素）。气泡在立绘上方展开，所以窗口偏高一点。
export const PET_WIDTH = 260;
export const PET_HEIGHT = 320;
export const PET_MARGIN = 24;

// 宠物缩放：只有一个数值，预设档位只是它的具名值（避免「预设 + 连续值」双轨）。
export const PET_SCALE_MIN = 0.4;
export const PET_SCALE_MAX = 2.0;
export const PET_SCALE_DEFAULT = 1;
export const PET_SCALE_PRESETS = [
  {id: 'small', label: '小', scale: 0.6},
  {id: 'medium', label: '中', scale: 1},
  {id: 'large', label: '大', scale: 1.5},
];

// 台词上限与 engine.mjs 的 say() 一致：60 字。
export const PET_SAY_MAX = 60;

// 宠物名册镜像 engine.mjs 的 PETS：states=true 表示按睡眠/心情切换四帧。
const PET_SPECIES = {
  libao: {sprite: 'libao', states: false},
  chestnut: {sprite: 'cat', states: true},
};
const DEFAULT_SPECIES = 'libao';

// 归一化缩放值：非有限数回落默认值，越界夹紧，四舍五入到 2 位。
export function petScaleClamp(value) {
  const n = Number(value);
  if (!Number.isFinite(n)) return PET_SCALE_DEFAULT;
  return Math.min(PET_SCALE_MAX, Math.max(PET_SCALE_MIN, Math.round(n * 100) / 100));
}

// 窗口几何的唯一来源：基准尺寸 × scale，贴 workArea 右下角。
// 多显示器时 workArea.x/y 可能非 0；缺字段按 0 处理，绝不抛异常。
export function petWindowBounds(workArea, scale) {
  const s = petScaleClamp(scale);
  const width = Math.max(1, Math.round(PET_WIDTH * s));
  const height = Math.max(1, Math.round(PET_HEIGHT * s));
  const area = workArea || {};
  const ax = Number.isFinite(area.x) ? area.x : 0;
  const ay = Number.isFinite(area.y) ? area.y : 0;
  const aw = Number.isFinite(area.width) ? area.width : 0;
  const ah = Number.isFinite(area.height) ? area.height : 0;
  return {x: ax + aw - width - PET_MARGIN, y: ay + ah - height - PET_MARGIN, width, height};
}

// BrowserWindow 选项（不含 webPreferences，由 main.mjs 注入 preload 等安全配置）。
// scale 缺省为 1，输出与本函数加缩放参数之前完全一致。
export function petWindowOptions(workArea, scale = 1) {
  const bounds = petWindowBounds(workArea, scale);
  return {
    width: bounds.width,
    height: bounds.height,
    x: bounds.x,
    y: bounds.y,
    frame: false,
    transparent: true,
    alwaysOnTop: true,
    skipTaskbar: true,
    resizable: false,
    focusable: false, // 常驻宠物不抢焦点
    hasShadow: false,
    show: false,
    title: 'szuDesktop 宠物',
  };
}

// 镜像 engine.mjs 的 say()：String(text).slice(0,60)。
export function petSay(text) {
  return String(text).slice(0, PET_SAY_MAX);
}

// 镜像 engine.mjs 的 petSprite()：荔宝单帧；栗栗按 sleeping/mood 四帧切换。
export function petSpriteFor(pet) {
  const spec = PET_SPECIES[pet?.species] || PET_SPECIES[DEFAULT_SPECIES];
  if (!spec.states) return spec.sprite;
  return pet.sleeping ? 'cat-sleep' : pet.mood < 35 ? 'cat-sad' : pet.mood > 65 ? 'cat-happy' : 'cat-normal';
}

// 从存档 game 段取当前伙伴，镜像 engine.mjs 的 activePet()。读不到就返回 null，
// 由调用方决定不推送（状态读不到时如实未知，不伪造）。
export function activePetOf(game) {
  const pets = game?.pets;
  if (!Array.isArray(pets) || pets.length === 0) return null;
  return pets[game.active] || pets[0] || null;
}

// 宠物窗 IPC 的可信来源校验，结构与 window-policy.mjs 的 isTrustedSender 一致：
// 只认宠物窗自己的主 frame，且 frame URL 恰好是本地 pet.html。
export function isPetSender(event, petWin, petUrl) {
  return Boolean(petWin && event.sender === petWin.webContents
    && event.senderFrame === petWin.webContents.mainFrame
    && event.senderFrame?.url === petUrl);
}

// 动作表：kind=once 表示播放一次后回落基础动作，不是无限循环。
// 动画全部由 pet.html 的 CSS keyframes 播放，这里的 duration 只用于渲染层排程。
export const PET_ACTIONS = {
  idle:  {label: '待机',   kind: 'loop', duration: 3200},
  happy: {label: '开心',   kind: 'loop', duration: 1200},
  sad:   {label: '难过',   kind: 'loop', duration: 2600},
  sleep: {label: '睡觉',   kind: 'loop', duration: 3600},
  blink: {label: '轻晃',   kind: 'once', duration: 260},
  yawn:  {label: '伸懒腰', kind: 'once', duration: 900},
  walk:  {label: '走动',   kind: 'once', duration: 1100},
  react: {label: '回应',   kind: 'once', duration: 700},
};

// 基础动作：完全由庭院存档的 sleeping/mood 推导，判断与 engine.mjs 的 petSprite() 一致。
// 读不到就回落 idle，不伪造。
export function petBaseAction(pet) {
  if (!pet) return 'idle';
  if (pet.sleeping) return 'sleep';
  const mood = Number(pet.mood);
  if (Number.isFinite(mood) && mood > 65) return 'happy';
  if (Number.isFinite(mood) && mood < 35) return 'sad';
  return 'idle';
}

// 一次性覆盖优先于基础动作；oneShot 不在动作表内时忽略。
export function petActionFor(pet, oneShot) {
  if (oneShot && Object.hasOwn(PET_ACTIONS, oneShot)) return oneShot;
  return petBaseAction(pet);
}

// 随机待机的候选池与权重。把「加权随机」变成 tick 的确定性函数，因此可在 Node 下单测。
const IDLE_POOL = ['blink', 'yawn', 'walk'];
export function petIdleWeights(pet) {
  const energy = Number(pet?.energy);
  return {
    blink: 5,
    yawn: Number.isFinite(energy) && energy < 30 ? 6 : 2,
    walk: pet?.sleeping ? 0 : 3,
    none: 6,
  };
}

// MurmurHash3 尾混合：保证连续 tick 在取模后仍均匀分布。
function mix32(n) {
  let h = (Number(n) | 0) + 0x9e3779b9;
  h = Math.imul(h ^ (h >>> 16), 0x85ebca6b);
  h = Math.imul(h ^ (h >>> 13), 0xc2b2ae35);
  return (h ^ (h >>> 16)) >>> 0;
}

// 返回三个一次性动作之一，或 null（这次不做动作，继续待机）。
export function petIdleAction(tick, pet) {
  const w = petIdleWeights(pet);
  const total = w.blink + w.yawn + w.walk + w.none;
  if (total <= 0) return null;
  let r = mix32(tick) % total;
  for (const id of IDLE_POOL) {
    if (r < w[id]) return id;
    r -= w[id];
  }
  return null;
}

// 缩放值命中哪个预设档位；自定义值与非有限值返回 null（托盘菜单据此决定要不要打勾）。
export function petPresetFor(scale) {
  const n = Number(scale);
  if (!Number.isFinite(n)) return null;
  const hit = PET_SCALE_PRESETS.find(p => p.scale === petScaleClamp(n));
  return hit ? hit.id : null;
}

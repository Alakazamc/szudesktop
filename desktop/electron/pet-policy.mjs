// 宠物窗的纯策略模块：不 import electron，便于像 window-policy.mjs 一样单测。
// 立绘与台词规则镜像 desktop/assets/garden/engine.mjs 的 petSprite()/say()。

// 窗口尺寸与右下角停靠留白（像素）。气泡在立绘上方展开，所以窗口偏高一点。
export const PET_WIDTH = 260;
export const PET_HEIGHT = 320;
export const PET_MARGIN = 24;

// 台词上限与 engine.mjs 的 say() 一致：60 字。
export const PET_SAY_MAX = 60;

// 宠物名册镜像 engine.mjs 的 PETS：states=true 表示按睡眠/心情切换四帧。
const PET_SPECIES = {
  libao: {sprite: 'libao', states: false},
  chestnut: {sprite: 'cat', states: true},
};
const DEFAULT_SPECIES = 'libao';

// BrowserWindow 选项（不含 webPreferences，由 main.mjs 注入 preload 等安全配置）。
// 位置按传入的 workArea 贴右下角，多显示器时 workArea.x/y 可能非 0。
export function petWindowOptions(workArea) {
  return {
    width: PET_WIDTH,
    height: PET_HEIGHT,
    x: workArea.x + workArea.width - PET_WIDTH - PET_MARGIN,
    y: workArea.y + workArea.height - PET_HEIGHT - PET_MARGIN,
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

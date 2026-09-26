// 宠物窗设置的本机持久化：scale 数值与可选的窗口位置。
// 放 Electron userData 而不进 Go workspace——宠物尺寸是外壳设置，与庭院游戏存档是两层关注点。
// 本模块不 import electron，只接收 userData 路径，因此可在纯 Node 下单测。
import {readFileSync, writeFileSync, renameSync, mkdirSync} from 'node:fs';
import {dirname} from 'node:path';
import {petScaleClamp, PET_SCALE_DEFAULT} from './pet-policy.mjs';

export const PET_SETTINGS_VERSION = 1;

export function petSettingsPath(userData) {
  return `${userData}/pet-settings.json`;
}

// 读不到就如实回落默认值：文件缺失、JSON 损坏、形状不对都不抛异常，
// 也绝不在读路径上改动或删除用户文件。
export function readPetSettings(userData) {
  try {
    const raw = JSON.parse(readFileSync(petSettingsPath(userData), 'utf8'));
    if (raw?.version !== PET_SETTINGS_VERSION) return {scale: PET_SCALE_DEFAULT};
    const scale = raw?.scale;
    if (typeof scale !== 'number' || !Number.isFinite(scale)) return {scale: PET_SCALE_DEFAULT};
    const settings = {scale: petScaleClamp(scale)};
    if (Number.isFinite(raw.position?.x) && Number.isFinite(raw.position?.y)) {
      settings.position = {x: raw.position.x, y: raw.position.y};
    }
    return settings;
  } catch {
    return {scale: PET_SCALE_DEFAULT};
  }
}

// 原子写：先写同目录临时文件再 rename，避免半截文件被下次启动读到。
export function writePetSettings(userData, scale, position) {
  const payload = {version: PET_SETTINGS_VERSION, scale: petScaleClamp(scale)};
  if (Number.isFinite(position?.x) && Number.isFinite(position?.y)) {
    payload.position = {x: position.x, y: position.y};
  }
  const file = petSettingsPath(userData);
  mkdirSync(dirname(file), {recursive: true});
  writeFileSync(`${file}.tmp`, JSON.stringify(payload, null, 2));
  renameSync(`${file}.tmp`, file);
  return payload;
}

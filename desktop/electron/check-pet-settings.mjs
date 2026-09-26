import assert from 'node:assert/strict';
import {mkdtempSync,readFileSync,writeFileSync,existsSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {readPetSettings,writePetSettings,petSettingsPath,PET_SETTINGS_VERSION} from './pet-settings.mjs';
import {PET_SCALE_DEFAULT,PET_SCALE_MIN,PET_SCALE_MAX} from './pet-policy.mjs';

const dir=mkdtempSync(join(tmpdir(),'szu-pet-settings-'));

// 文件不存在：回落默认值，不抛异常。
assert.deepEqual(readPetSettings(dir),{scale:PET_SCALE_DEFAULT});

// 写入后能读回，且落盘内容是 {version, scale}。
const written=writePetSettings(dir,1.5);
assert.equal(written.version,PET_SETTINGS_VERSION);
assert.equal(written.scale,1.5);
assert.deepEqual(readPetSettings(dir),{scale:1.5});
const onDisk=JSON.parse(readFileSync(petSettingsPath(dir),'utf8'));
assert.deepEqual(onDisk,{version:1,scale:1.5});

// 写入时会归一化：越界值夹紧后才落盘。
writePetSettings(dir,9);
assert.deepEqual(readPetSettings(dir),{scale:PET_SCALE_MAX});
writePetSettings(dir,0.01);
assert.deepEqual(readPetSettings(dir),{scale:PET_SCALE_MIN});
writePetSettings(dir,NaN);
assert.deepEqual(readPetSettings(dir),{scale:PET_SCALE_DEFAULT});

// 损坏文件：JSON 语法错误 → 回落默认值，且不能把原文件删掉。
writeFileSync(petSettingsPath(dir),'{ 这不是 json');
assert.deepEqual(readPetSettings(dir),{scale:PET_SCALE_DEFAULT});
assert.ok(existsSync(petSettingsPath(dir)),'损坏的存档必须原样保留');

// 形状不对：数组、null、scale 是字符串、版本不符 → 一律回落默认值。
for(const bad of ['[]','null','{"scale":"abc"}','{"version":2,"scale":1.5}','{"scale":true}','{"scale":1.5,"extra":1}']){
  writeFileSync(petSettingsPath(dir),bad);
  assert.deepEqual(readPetSettings(dir),{scale:PET_SCALE_DEFAULT},bad);
}

// 目录不存在时写入要自建目录。
const nested=join(dir,'a','b','c');
writePetSettings(nested,0.6);
assert.deepEqual(readPetSettings(nested),{scale:0.6});

// 原子写：成功后不应留下 .tmp 残留文件。
assert.ok(!existsSync(petSettingsPath(dir)+'.tmp'),'原子写成功后不应留下临时文件');

console.log('Pet settings: atomic read/write, normalization and corruption fallback passed');

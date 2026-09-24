import {execFileSync} from 'node:child_process';
import {readFileSync, writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const here=path.dirname(fileURLToPath(import.meta.url));
const repoRoot=path.resolve(here,'..','..');

// 单一来源版本号（不另写死）。仓库里的 package.json 恒为 0.0.0，本脚本不改它，
//    版本通过 electron-builder 的 extraMetadata 在打包时注入（见第 3 步）。
const ver=readFileSync(path.join(repoRoot,'internal','version','VERSION'),'utf8').trim();
if(!/^(?:beta|v)?\d+\.\d+\.\d+$/.test(ver)){
  throw new Error('VERSION 必须是 betaX.Y.Z、vX.Y.Z 或 X.Y.Z');
}
const semver=ver.replace(/^(?:beta|v)/,'');

// 编 Go sidecar（CI 已单独构建并冒烟时可复用同一份产物）。
if(!process.argv.includes('--skip-sidecar')){
  execFileSync('python',[path.join(repoRoot,'desktop','build-windows.py')],{cwd:repoRoot,stdio:'inherit'});
}

// 3. 打包。不走 npx：Windows 上 Node 拒绝 execFileSync 直接 spawn .cmd（EINVAL），
//    用当前 node 跑本地装好的 electron-builder CLI，等价于 npx electron-builder。
const ebCli=path.join(here,'node_modules','electron-builder','cli.js');
execFileSync(process.execPath,[ebCli,'--win','--x64','--publish','never','--config','electron-builder.yml',
  `--config.extraMetadata.version=${semver}`],{cwd:here,stdio:'inherit'});
const installer=path.join(here,'release',`szuDesktop-Setup-${semver}.exe`);
const digest=createHash('sha256').update(readFileSync(installer)).digest('hex');
writeFileSync(installer+'.sha256',`${digest}  ${path.basename(installer)}\n`,'ascii');
console.log('\n完成。安装包在 desktop/electron/release/，版本 '+ver);

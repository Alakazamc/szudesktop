import {execFileSync} from 'node:child_process';
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const here=path.dirname(fileURLToPath(import.meta.url));
const repoRoot=path.resolve(here,'..','..');

// 1. 编 Go sidecar（复用现有脚本：同步页面 + 编译 + 打图标/版本）
execFileSync('python',[path.join(repoRoot,'desktop','build-windows.py')],{cwd:repoRoot,stdio:'inherit'});

// 2. 用单一来源版本号覆盖 package.json 的 version（不另写死）
const ver=readFileSync(path.join(repoRoot,'internal','version','VERSION'),'utf8').trim();
if(!ver){console.error('VERSION 为空');process.exit(1);}
const pkgPath=path.join(here,'package.json');
const pkg=JSON.parse(readFileSync(pkgPath,'utf8'));
pkg.version=ver.replace(/^beta/,''); // electron-builder 要 x.y.z；beta0.7.3 -> 0.7.3
writeFileSync(pkgPath,JSON.stringify(pkg,null,2)+'\n');

// 3. 打包。不走 npx：Windows 上 Node 拒绝 execFileSync 直接 spawn .cmd（EINVAL），
//    用当前 node 跑本地装好的 electron-builder CLI，等价于 npx electron-builder。
const ebCli=path.join(here,'node_modules','electron-builder','cli.js');
execFileSync(process.execPath,[ebCli,'--win','--config','electron-builder.yml'],{cwd:here,stdio:'inherit'});
console.log('\n完成。安装包在 desktop/electron/release/，版本 '+ver);

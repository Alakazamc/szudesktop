import {app, BrowserWindow, shell} from 'electron';
import {writeFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {startSidecar, stopSidecar} from './sidecar.mjs';
import {isSafeExternalUrl} from './external-url.mjs';

// dev: 仓库里 dist/ 的 exe；打包后: resources 里的 extraResource
function sidecarCommand(){
  const exe=process.platform==='win32'?'szudesktop-windows-amd64.exe':'szudesktop';
  if(app.isPackaged) return {command:path.join(process.resourcesPath,exe),args:['--no-open']};
  const here=path.dirname(fileURLToPath(import.meta.url)); // desktop/electron
  const repoRoot=path.resolve(here,'..','..');
  return {command:path.join(repoRoot,'dist',exe),args:['--no-open']};
}

let handle=null, mainWin=null;

async function boot(){
  const {command,args}=sidecarCommand();
  handle=await startSidecar({command,args});
  mainWin=new BrowserWindow({width:1200,height:820,title:'szuDesktop',
    webPreferences:{contextIsolation:true,nodeIntegration:false}});
  // 外链交给系统浏览器（仅 http/https），不在应用内开新窗
  mainWin.webContents.setWindowOpenHandler(({url})=>{ if(isSafeExternalUrl(url)) shell.openExternal(url); return {action:'deny'}; });
  await mainWin.loadURL(handle.baseUrl);
  if (process.env.SZU_SHOT) {
    await new Promise((r) => setTimeout(r, 1500)); // 等 SPA 首屏渲染
    const img = await mainWin.webContents.capturePage();
    writeFileSync(process.env.SZU_SHOT, img.toPNG());
    console.log('SHOT_WRITTEN', process.env.SZU_SHOT);
    await stopSidecar(handle.child);
    app.quit();
    return;
  }
  mainWin.on('closed',()=>{mainWin=null;});
}

const gotLock=app.requestSingleInstanceLock();
if(!gotLock){ app.quit(); }
else{
  app.on('second-instance',()=>{ if(mainWin){ if(mainWin.isMinimized())mainWin.restore(); mainWin.focus(); }});
  app.whenReady().then(boot).catch((e)=>{ console.error('启动失败',e); app.quit(); });
  app.on('window-all-closed',async()=>{ if(handle)await stopSidecar(handle.child); app.quit(); });
  app.on('before-quit',async()=>{ if(handle)await stopSidecar(handle.child); });
}

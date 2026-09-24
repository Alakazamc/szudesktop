import {app, BrowserWindow, dialog, ipcMain, shell} from 'electron';
import {writeFileSync, mkdirSync, renameSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {startSidecar} from './sidecar.mjs';
import {isSafeExternalUrl} from './external-url.mjs';
import {contentSecurityPolicy,isAppUrl,isTrustedSender} from './window-policy.mjs';

const here=path.dirname(fileURLToPath(import.meta.url));
// Installation smoke runs use their own profile and Go state, never the user's account.
const smokeReport=process.env.SZU_SMOKE_REPORT;
const smoke=Boolean(smokeReport && path.isAbsolute(smokeReport)
  && process.env.SZUNET_CONFIG_DIR && path.isAbsolute(process.env.SZUNET_CONFIG_DIR));
if(smoke) app.setPath('userData',path.join(process.env.SZUNET_CONFIG_DIR,'electron-profile'));

function sidecarCommand(){
  const exe=process.platform==='win32'?'szudesktop-windows-amd64.exe':'szudesktop';
  const command=app.isPackaged?path.join(process.resourcesPath,exe):path.resolve(here,'..','..','dist',exe);
  return {command,args:['--no-open',...(smoke?['--no-auto-login']:[])]};
}
let handle=null,mainWin=null,quitting=false,quitReady=false,shutdownPromise=null,healthTimer=null,failureShown=false;
let startup=null;

async function engineFailed(message){
  if(quitting||failureShown)return;
  failureShown=true;
  const options={type:'error',title:'szuDesktop 引擎已停止',message:'本机服务暂时无法使用',
    detail:message+'。已保存的庭院和学习记录会保留。',buttons:['重新打开','退出'],defaultId:0,cancelId:1};
  const result=await (mainWin&&!mainWin.isDestroyed()?dialog.showMessageBox(mainWin,options):dialog.showMessageBox(options));
  if(result.response===0)app.relaunch();
  app.quit();
}
function openExternal(url){
  if(isSafeExternalUrl(url)) void shell.openExternal(url).catch(()=>{
    if(!quitting)dialog.showErrorBox('未能打开浏览器','请检查系统默认浏览器设置后重试。');
  });
}
async function recordSmoke(){
  if(!smoke)return;
  const deadline=Date.now()+15000;
  let rendered=false;
  while(Date.now()<deadline){
    rendered=await mainWin.webContents.executeJavaScript(`Boolean(document.querySelector('#nav [data-action="navigate"]') && document.querySelector('#main #network-summary') && window.szuDesktop?.shell === 'electron')`);
    if(rendered)break;
    await new Promise(r=>setTimeout(r,100));
  }
  if(!rendered)throw Error('安装版主界面或隔离接口没有就绪');
  const response=await fetch(handle.baseUrl+'/api/status',{signal:AbortSignal.timeout(5000)});
  if(!response.ok)throw Error('安装版引擎健康检查失败');
  const status=await response.json();
  if(process.env.SZU_SMOKE_SCREENSHOT && path.isAbsolute(process.env.SZU_SMOKE_SCREENSHOT)){
    await mainWin.webContents.executeJavaScript('document.fonts.ready.then(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))');
    const shot=await mainWin.webContents.capturePage();
    writeFileSync(process.env.SZU_SMOKE_SCREENSHOT,shot.toPNG());
  }
  mkdirSync(path.dirname(smokeReport),{recursive:true});
  writeFileSync(smokeReport+'.tmp',JSON.stringify({version:status.app_version,packageVersion:app.getVersion(),electron:process.versions.electron,
    appPid:process.pid,sidecarPid:handle.owned?handle.child.pid:null,owned:handle.owned,
    baseUrl:handle.baseUrl,title:mainWin.getTitle(),rendered},null,2));
  renameSync(smokeReport+'.tmp',smokeReport);
  if(process.env.SZU_SMOKE_QUIT_AFTER_REPORT==='1')app.quit();
}
async function boot(){
  handle=await startSidecar(sidecarCommand());
  if(quitting)return;
  mainWin=new BrowserWindow({width:1200,height:820,minWidth:380,minHeight:480,show:false,title:'szuDesktop',
    webPreferences:{preload:path.join(here,'preload.cjs'),contextIsolation:true,nodeIntegration:false,sandbox:true}});
  mainWin.setMenuBarVisibility(false);
  const wc=mainWin.webContents;
  wc.session.setPermissionRequestHandler((_wc,_permission,callback)=>callback(false));
  wc.session.setPermissionCheckHandler(()=>false);
  wc.session.webRequest.onHeadersReceived((details,callback)=>{
    const headers={...details.responseHeaders};
    if(isAppUrl(details.url,handle.baseUrl))headers['Content-Security-Policy']=[contentSecurityPolicy];
    callback({responseHeaders:headers});
  });
  wc.setWindowOpenHandler(({url})=>{openExternal(url);return {action:'deny'};});
  wc.on('will-navigate',(event,url)=>{if(!isAppUrl(url,handle.baseUrl)){event.preventDefault();openExternal(url);}});
  wc.on('will-redirect',(event,url)=>{if(!isAppUrl(url,handle.baseUrl))event.preventDefault();});
  wc.on('render-process-gone',()=>void engineFailed('窗口进程意外结束，请重新打开应用'));
  if(handle.owned){
    handle.child.once('exit',()=>void engineFailed('后台引擎意外结束，请重新打开应用'));
  }else{
    // The reused service belongs to another launcher; never terminate it on our exit.
    let checking=false;
    healthTimer=setInterval(async()=>{
      if(checking||quitting)return;
      checking=true;
      try{const r=await fetch(handle.baseUrl+'/',{signal:AbortSignal.timeout(2500)});if(!r.ok)throw Error();}
      catch{void engineFailed('此前已运行的后台服务已停止');}
      finally{checking=false;}
    },5000);
    healthTimer.unref();
  }
  mainWin.on('closed',()=>{mainWin=null;});
  await mainWin.loadURL(handle.baseUrl);
  mainWin.show();
  await recordSmoke();
}

const gotLock=app.requestSingleInstanceLock();
if(!gotLock)app.quit();
else{
  ipcMain.handle('szu:quit',(event)=>{
    if(!isTrustedSender(event,mainWin,handle?.baseUrl))throw Error('请求来源不匹配');
    setImmediate(()=>app.quit());
    return true;
  });
  app.on('second-instance',()=>{if(mainWin&&!mainWin.isDestroyed()){if(mainWin.isMinimized())mainWin.restore();mainWin.show();mainWin.focus();}});
  app.whenReady().then(()=>{
    startup=boot();
    return startup;
  }).catch(e=>{
    if(smoke){mkdirSync(path.dirname(smokeReport),{recursive:true});writeFileSync(smokeReport,JSON.stringify({error:e.message}));}
    else if(!quitting)dialog.showErrorBox('szuDesktop 启动失败',e.message+'。请重新打开应用；若仍失败，请重新安装。');
    app.quit();
  });
  app.on('window-all-closed',()=>app.quit());
  app.on('before-quit',event=>{
    if(quitReady)return;
    event.preventDefault();
    quitting=true;
    clearInterval(healthTimer);
    if(!shutdownPromise)shutdownPromise=(async()=>{
      try{await startup;}catch{}
      // Close our renderer first so its event stream cannot delay Go's graceful shutdown.
      if(mainWin&&!mainWin.isDestroyed())mainWin.destroy();
      try{if(handle)await handle.stop();}
      catch{if(!smoke)dialog.showErrorBox('后台服务未正常退出','请稍后重试；不要重复运行安装程序。');}
      finally{quitReady=true;app.quit();}
    })();
  });
}

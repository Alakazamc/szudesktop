import {app, BrowserWindow, dialog, ipcMain, Menu, screen, shell, Tray} from 'electron';
import {writeFileSync, mkdirSync, renameSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {startSidecar} from './sidecar.mjs';
import {isSafeExternalUrl} from './external-url.mjs';
import {contentSecurityPolicy,isAppUrl,isTrustedSender} from './window-policy.mjs';
import {petWindowOptions,petWindowBounds,petScaleClamp,petPresetFor,petActionFor,petSay,petSpriteFor,activePetOf,isPetSender,PET_SCALE_DEFAULT,PET_SCALE_PRESETS,PET_WIDTH,PET_HEIGHT} from './pet-policy.mjs';
import {readPetSettings,writePetSettings} from './pet-settings.mjs';
import {createSchoolWindows} from './school-window.mjs';
import {isSchoolURL} from './school-policy.mjs';

const here=path.dirname(fileURLToPath(import.meta.url));
// 菜单与主界面共用同一份庭院规则，包含离线成长；打包时直接复制源模块。
const gardenEngine=import(pathToFileURL(app.isPackaged
  ?path.join(process.resourcesPath,'garden-engine.mjs')
  :path.resolve(here,'..','assets','garden','engine.mjs')).href);
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
let petWin=null,tray=null,trayMenu=null,petTimer=null,petGreeted=false,petHtmlUrl=null,petScale=PET_SCALE_DEFAULT;
let petMenu=null,petGame=null,petPosition=null,petDrag=null;
let startup=null,schoolWindows=null;
const smokeErrors=[];

// 宠物窗要显示的图标；开发/打包路径解析方式与 sidecarCommand() 保持一致。
function petIconPath(){
  return app.isPackaged?path.join(process.resourcesPath,'szudesktop.ico'):path.resolve(here,'..','assets','szudesktop.ico');
}
function showMainWindow(){
  if(mainWin&&!mainWin.isDestroyed()){
    if(mainWin.isMinimized())mainWin.restore();
    mainWin.show();mainWin.focus();
  }
}
function sendPet(channel,payload){
  if(petWin&&!petWin.isDestroyed())petWin.webContents.send(channel,payload);
}
// 从 sidecar 拉庭院存档，推导当前伙伴立绘并推给宠物窗。
// 全程 try/catch：sidecar 短暂不可用不能拖垮主进程；读不到就如实不推，不伪造状态。
async function pushPetState(){
  if(quitting||!handle||!petWin||petWin.isDestroyed())return;
  try{
    const response=await fetch(handle.baseUrl+'/api/workspace',{signal:AbortSignal.timeout(5000)});
    if(!response.ok)return;
    const snapshot=await response.json();
    if(!snapshot.data)return;
    const {settle,normalize}=await gardenEngine;
    const game=settle(normalize(snapshot.data)).game;
    const pet=activePetOf(game);
    if(!pet)return;
    petGame=game;
    sendPet('pet:state',petSpriteFor(pet));
    sendPet('pet:action',{id:petActionFor(pet,null),energy:Number(pet.energy),sleeping:Boolean(pet.sleeping)});
    if(!petGreeted){
      petGreeted=true;
      const name=pet.name||'伙伴';
      sendPet('pet:say',petSay(`我是${name}。点击我打开菜单，拖动我换个位置。`));
    }
  }catch{}
}
async function createPetWindow(){
  if(quitting)return;
  const {workArea}=petPosition?screen.getDisplayNearestPoint(petPosition):screen.getPrimaryDisplay();
  petHtmlUrl=pathToFileURL(path.join(here,'pet.html')).href;
  petWin=new BrowserWindow({...petWindowOptions(workArea,petScale,petPosition),
    webPreferences:{preload:path.join(here,'pet-preload.cjs'),contextIsolation:true,nodeIntegration:false,sandbox:true}});
  petWin.setMenuBarVisibility(false);
  const pwc=petWin.webContents;
  if(smoke){
    pwc.on('preload-error',(_event,_file,error)=>{if(smokeErrors.length<10)smokeErrors.push('pet preload: '+error.message);});
    pwc.on('console-message',details=>{if(details.level==='error'&&smokeErrors.length<10)smokeErrors.push('pet: '+details.message);});
  }
  // 宠物窗不加载任何远程内容：拦截一切导航与新窗请求。
  pwc.setWindowOpenHandler(()=>({action:'deny'}));
  pwc.on('will-navigate',event=>event.preventDefault());
  pwc.on('will-redirect',event=>event.preventDefault());
  petWin.on('closed',()=>{petWin=null;});
  await petWin.loadFile(path.join(here,'pet.html'));
  if(quitting||!petWin||petWin.isDestroyed())return;
  petWin.setAlwaysOnTop(true,'screen-saver');
  petWin.show();
  syncPetGeometry();
  // 首帧推送：立绘 + 招呼台词，随后每 ~30s 刷新一次。
  await pushPetState();
  const petShot=process.env.SZU_PET_SHOT;
  if(petShot&&path.isAbsolute(petShot)){
    await pwc.executeJavaScript('document.fonts.ready.then(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))))');
    mkdirSync(path.dirname(petShot),{recursive:true});
    writeFileSync(petShot,(await pwc.capturePage()).toPNG());
  }
  if(!petTimer){petTimer=setInterval(()=>void pushPetState(),30000);petTimer.unref();}
}
// 应用缩放：归一化 → 持久化 → 重算窗口几何 → 推送渲染器 → 刷新托盘勾选。
// 只走程序化 setBounds，绝不开原生 resizable（透明窗原生缩放在 Windows 上不可靠）。
function applyPetScale(scale){
  const next=petScaleClamp(scale);
  if(petWin&&!petWin.isDestroyed()){
    const previous=petWin.getBounds(),{workArea}=screen.getDisplayMatching(previous);
    const size=petWindowBounds(workArea,next);
    const position={x:previous.x+(previous.width-size.width)/2,y:previous.y+previous.height-size.height};
    const bounds=petWindowBounds(workArea,next,position);
    writePetSettings(app.getPath('userData'),next,{x:bounds.x,y:bounds.y});
    petScale=next;
    petWin.setBounds(bounds);
    syncPetGeometry();
  }else{
    writePetSettings(app.getPath('userData'),next,petPosition);
    petScale=next;
  }
  refreshTrayMenu();
  if(mainWin&&!mainWin.isDestroyed())mainWin.webContents.send('szu:pet-scale',petScale);
  return petScale;
}
function syncPetGeometry(){
  if(!petWin||petWin.isDestroyed())return;
  const bounds=petWin.getBounds();
  petPosition={x:bounds.x,y:bounds.y};
  sendPet('pet:scale',Math.min(petScale,bounds.width/PET_WIDTH,bounds.height/PET_HEIGHT));
}
function changePetSize(scale){
  try{applyPetScale(scale);}catch{sendPet('pet:say','大小没有保存成功，请检查本机配置目录后再试。');}
}
function dispatchPetCommand(command){
  if(!mainWin||mainWin.isDestroyed())return;
  if(['home','garden','farm','study'].includes(command))showMainWindow();
  mainWin.webContents.send('szu:pet-command',command);
}
async function openPetMenu(){
  if(!petWin||petWin.isDestroyed()||quitting)return;
  await pushPetState();
  if(!petWin||petWin.isDestroyed()||quitting)return;
  const pet=activePetOf(petGame);
  const care=(label,command)=>({id:command,label,enabled:Boolean(pet),click:()=>dispatchPetCommand(command)});
  petMenu=Menu.buildFromTemplate([
    {label:pet?`${pet.name} · Lv.${Math.min(20,1+Math.floor(pet.xp/50))}`:'伙伴状态读取中',enabled:false},
    ...(pet?[{label:`饱腹 ${Math.round(pet.hunger)} · 心情 ${Math.round(pet.mood)} · 精力 ${Math.round(pet.energy)}`,enabled:false}]:[]),
    {type:'separator'},
    care('摸摸头','pat'),care(`喂食${petGame?`（剩余 ${petGame.food} 份）`:''}`,'feed'),care('陪它玩','play'),care(pet?.sleeping?'叫醒伙伴':'让它睡一会','sleep'),
    {label:'切换伙伴',enabled:Boolean(pet),submenu:(petGame?.pets||[]).map((companion,index)=>({
      id:`switchPet:${index}`,label:companion.name,type:'radio',checked:index===petGame.active,
      click:()=>{if(index!==petGame.active)dispatchPetCommand(`switchPet:${index}`);},
    }))},
    {type:'separator'},
    {id:'garden',label:'看看庭院',click:()=>dispatchPetCommand('garden')},
    {id:'farm',label:'照看农田',click:()=>dispatchPetCommand('farm')},
    {id:'study',label:'学习与专注',click:()=>dispatchPetCommand('study')},
    {label:`宠物大小（${Math.round(petScale*100)}%）`,submenu:[
      {label:'缩小一点',enabled:petScale>0.4,click:()=>changePetSize(petScale-0.1)},
      {label:'放大一点',enabled:petScale<2,click:()=>changePetSize(petScale+0.1)},
      {type:'separator'},...petSizeMenu(),
      {label:'恢复默认大小',click:()=>changePetSize(1)},
    ]},
    {type:'separator'},
    {id:'home',label:'打开主窗口',click:()=>dispatchPetCommand('home')},
    {label:'隐藏宠物',click:()=>{petWin?.hide();refreshTrayMenu();}},
    {label:'退出应用',click:()=>app.quit()},
  ]);
  petWin.setFocusable(true);
  petMenu.popup({window:petWin,callback:()=>{if(petWin&&!petWin.isDestroyed())petWin.setFocusable(false);}});
}
function petSizeMenu(){
  const current=petPresetFor(petScale);
  return PET_SCALE_PRESETS.map(p=>({
    label:`${p.label}（${Math.round(p.scale*100)}%）`,
    type:'checkbox',
    checked:current===p.id,
    click:()=>{try{applyPetScale(p.scale);}catch{refreshTrayMenu();dialog.showErrorBox('未能保存宠物大小','请检查本机配置目录是否可写，然后重试。');}},
  }));
}
function refreshTrayMenu(){
  if(!tray)return;
  const visible=Boolean(petWin&&!petWin.isDestroyed()&&petWin.isVisible());
  trayMenu=Menu.buildFromTemplate([
    {label:visible?'隐藏宠物':'显示宠物',click:()=>{
      if(petWin&&!petWin.isDestroyed()){
        if(petWin.isVisible())petWin.hide();
        else{petWin.show();petWin.setAlwaysOnTop(true,'screen-saver');}
        refreshTrayMenu();
      }else void createPetWindow().then(refreshTrayMenu).catch(()=>{});
    }},
    {label:'宠物大小',submenu:petSizeMenu()},
    {label:'打开主窗口',click:showMainWindow},
    {type:'separator'},
    {label:'退出',click:()=>app.quit()},
  ]);
  tray.setContextMenu(trayMenu);
}
function createTray(){
  try{
    tray=new Tray(petIconPath());
    tray.setToolTip('szuDesktop 荔枝庭院');
    tray.on('click',showMainWindow);
    refreshTrayMenu();
    // 调试取证用：仅在显式开启宠物截图调试时打印，正常启动保持安静。
    if(process.env.SZU_PET_SHOT)console.log('[pet] tray created:',petIconPath());
  }catch(e){
    tray=null;
    if(smoke&&smokeErrors.length<10)smokeErrors.push('tray: '+e.message);
    else if(process.env.SZU_PET_SHOT)console.warn('[pet] tray failed:',e.message);
  }
}
async function startPet(){
  try{
    await createPetWindow();
    createTray();
  }catch(e){
    // 宠物窗是增强功能，失败绝不能拖垮主界面。
    if(smoke&&smokeErrors.length<10)smokeErrors.push('pet: '+e.message);
  }
}

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
  if(schoolWindows&&isSchoolURL(url)){
    void schoolWindows.open(url).catch(()=>dialog.showErrorBox('学校页面暂时无法打开','请检查校园网或 WebVPN；可通过学校窗口菜单在系统浏览器中打开。'));
    return;
  }
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
  const response=await fetch(handle.baseUrl+'/api/health',{signal:AbortSignal.timeout(5000)});
  if(!response.ok)throw Error('安装版引擎健康检查失败');
  const status=await response.json();
  const {checkPetRuntime}=await import('./smoke-pet.mjs');
  const pet=await checkPetRuntime({mainWin,petWin,tray,getMenu:()=>trayMenu,screen,
    getPetMenu:()=>petMenu,initialScale:petScale,userData:app.getPath('userData'),evidenceDir:path.dirname(smokeReport),baseUrl:handle.baseUrl});
  if(smokeErrors.length)throw Error(smokeErrors.join('; '));
  if(process.env.SZU_SMOKE_SCREENSHOT && path.isAbsolute(process.env.SZU_SMOKE_SCREENSHOT)){
    await mainWin.webContents.executeJavaScript('document.fonts.ready.then(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))');
    const shot=await mainWin.webContents.capturePage();
    writeFileSync(process.env.SZU_SMOKE_SCREENSHOT,shot.toPNG());
  }
  mkdirSync(path.dirname(smokeReport),{recursive:true});
  writeFileSync(smokeReport+'.tmp',JSON.stringify({version:status.app_version,packageVersion:app.getVersion(),electron:process.versions.electron,
    appPid:process.pid,sidecarPid:handle.owned?handle.child.pid:null,owned:handle.owned,
    baseUrl:handle.baseUrl,title:mainWin.getTitle(),rendered,pet},null,2));
  renameSync(smokeReport+'.tmp',smokeReport);
  if(process.env.SZU_SMOKE_QUIT_AFTER_REPORT==='1')app.quit();
}
async function boot(){
  handle=await startSidecar(sidecarCommand());
  if(quitting)return;
  schoolWindows=createSchoolWindows(()=>handle.baseUrl);
  mainWin=new BrowserWindow({width:1200,height:820,minWidth:380,minHeight:480,show:false,title:'szuDesktop',
    webPreferences:{preload:path.join(here,'preload.cjs'),contextIsolation:true,nodeIntegration:false,sandbox:true}});
  mainWin.setMenuBarVisibility(false);
  const wc=mainWin.webContents;
  if(smoke){
    wc.on('preload-error',(_event,_file,error)=>{if(smokeErrors.length<10)smokeErrors.push('preload: '+error.message);});
    wc.on('console-message',details=>{if(details.level==='error'&&smokeErrors.length<10)smokeErrors.push(details.message);});
  }
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
      try{const r=await fetch(handle.baseUrl+'/api/health',{signal:AbortSignal.timeout(2500)});if(!r.ok)throw Error();const health=await r.json();if(!health.ok||health.app!=='szuDesktop')throw Error();}
      catch{void engineFailed('此前已运行的后台服务已停止');}
      finally{checking=false;}
    },5000);
    healthTimer.unref();
  }
  mainWin.on('close',event=>{
    if(!quitting&&tray&&!tray.isDestroyed()){event.preventDefault();mainWin.hide();}
    else if(!quitting)app.quit();
  });
  mainWin.on('closed',()=>{mainWin=null;});
  await mainWin.loadURL(handle.baseUrl);
  mainWin.show();
  const petSettings=readPetSettings(app.getPath('userData'));
  petScale=petSettings.scale;petPosition=petSettings.position||null;
  if(!quitting)await startPet();
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
  // 主窗设置和宠物菜单共用缩放；所有 IPC 只接受对应本地窗口的主 frame。
  ipcMain.handle('szu:pet-scale-get',event=>{
    if(!isTrustedSender(event,mainWin,handle?.baseUrl))throw Error('请求来源不匹配');
    return petScale;
  });
  ipcMain.handle('szu:pet-scale-set',(event,value)=>{
    if(!isTrustedSender(event,mainWin,handle?.baseUrl))throw Error('请求来源不匹配');
    return applyPetScale(value);
  });
  for(const [channel,method] of [['szu:school-open','open'],['szu:school-sync','sync'],['szu:school-clear','clear']]){
    ipcMain.handle(channel,(event,value)=>{
      if(!isTrustedSender(event,mainWin,handle?.baseUrl))throw Error('请求来源不匹配');
      return schoolWindows[method](value);
    });
  }
  ipcMain.on('szu:pet-result',async(event,result)=>{
    if(!isTrustedSender(event,mainWin,handle?.baseUrl)||typeof result?.ok!=='boolean'||typeof result.message!=='string')return;
    await pushPetState();
    sendPet('pet:say',petSay(result.message));
    if(result.ok)sendPet('pet:react');
  });
  ipcMain.on('pet:menu',(event)=>{
    if(!petHtmlUrl||!isPetSender(event,petWin,petHtmlUrl))return;
    void openPetMenu();
  });
  ipcMain.on('pet:scale-step',(event,direction)=>{
    if(!petHtmlUrl||!isPetSender(event,petWin,petHtmlUrl)||![1,-1].includes(direction))return;
    changePetSize(petScale+direction*0.1);
  });
  ipcMain.on('pet:drag',(event,phase,point)=>{
    if(!petHtmlUrl||!isPetSender(event,petWin,petHtmlUrl)||!Number.isFinite(point?.x)||!Number.isFinite(point?.y))return;
    if(phase==='start')petDrag={cursor:point,bounds:petWin.getBounds(),moved:false};
    else if(phase==='move'&&petDrag){
      const {workArea}=screen.getDisplayNearestPoint(point);
      const position={x:petDrag.bounds.x+point.x-petDrag.cursor.x,y:petDrag.bounds.y+point.y-petDrag.cursor.y};
      petWin.setBounds(petWindowBounds(workArea,petScale,position));
      petDrag.moved=true;syncPetGeometry();
    }else if(phase==='end'&&petDrag){
      if(petDrag.moved){try{writePetSettings(app.getPath('userData'),petScale,petPosition);}catch{sendPet('pet:say','位置没有保存成功，下次打开可能回到原处。');}}
      petDrag=null;
    }
  });
  app.on('second-instance',showMainWindow);
  app.whenReady().then(()=>{
    const reposition=()=>{
      if(petWin&&!petWin.isDestroyed()){
        const {workArea}=screen.getDisplayMatching(petWin.getBounds());
        petWin.setBounds(petWindowBounds(workArea,petScale,petPosition));
        syncPetGeometry();
      }
    };
    screen.on('display-metrics-changed',reposition);
    screen.on('display-removed',reposition);
    screen.on('display-added',reposition);
    startup=boot();
    return startup;
  }).catch(async e=>{
    if(smoke){
      const report={error:e.message,consoleErrors:smokeErrors};
      try{
        if(mainWin&&!mainWin.isDestroyed()){
          report.page=await mainWin.webContents.executeJavaScript(`({url:location.href,shell:window.szuDesktop?.shell,navCount:document.querySelectorAll('#nav [data-action="navigate"]').length,mainText:document.querySelector('#main')?.innerText.slice(0,1500)})`);
          report.moduleType=await mainWin.webContents.executeJavaScript(`fetch('/assets/garden/app.mjs').then(r=>({status:r.status,type:r.headers.get('content-type')}))`);
          const shotPath=process.env.SZU_SMOKE_SCREENSHOT;
          if(shotPath&&path.isAbsolute(shotPath))writeFileSync(shotPath,(await mainWin.webContents.capturePage()).toPNG());
        }
      }catch(snapshotError){report.snapshotError=snapshotError.message;}
      mkdirSync(path.dirname(smokeReport),{recursive:true});writeFileSync(smokeReport,JSON.stringify(report,null,2));
    }
    else if(!quitting)dialog.showErrorBox('szuDesktop 启动失败',e.message+'。请重新打开应用；若仍失败，请重新安装。');
    app.quit();
  });
  app.on('window-all-closed',()=>app.quit());
  app.on('before-quit',event=>{
    if(quitReady)return;
    event.preventDefault();
    quitting=true;
    clearInterval(healthTimer);
    clearInterval(petTimer);petTimer=null;
    if(!shutdownPromise)shutdownPromise=(async()=>{
      try{await startup;}catch{}
      // Close our renderer first so its event stream cannot delay Go's graceful shutdown.
      if(petWin&&!petWin.isDestroyed())petWin.destroy();
      try{await schoolWindows?.shutdown();}
      catch{if(handle&&!handle.owned&&!smoke)dialog.showErrorBox('学校登录未能清除','请在仍运行的便携版中清除学校登录，或退出该后台服务。');}
      if(tray){tray.destroy();tray=null;}
      if(mainWin&&!mainWin.isDestroyed())mainWin.destroy();
      try{if(handle)await handle.stop();}
      catch{if(!smoke)dialog.showErrorBox('后台服务未正常退出','请稍后重试；不要重复运行安装程序。');}
      finally{quitReady=true;app.quit();}
    })();
  });
}

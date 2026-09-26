import {BrowserWindow,Menu,dialog,session,shell} from 'electron';
import {academicCookies,isSchoolURL,schoolTargets} from './school-policy.mjs';

// No persist: prefix: school cookies/cache disappear when the app exits.
export function createSchoolWindows(getBaseURL){
  let window=null,imported=false;
  const profile=session.fromPartition('szu-official',{cache:false});
  profile.setPermissionRequestHandler((_wc,_permission,done)=>done(false));
  profile.setPermissionCheckHandler(()=>false);
  async function local(endpoint,body,method='POST'){
    const response=await fetch(getBaseURL()+endpoint,{method,headers:{'Content-Type':'application/json'},
      body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(30000)});
    const result=await response.json();
    if(!response.ok)throw Error(result.error||'学校登录尚未完成，请在学校页面登录后重试');
    return result;
  }
  async function clear(){
    // Close remote pages first so their scripts cannot refresh a cleared session.
    if(window&&!window.isDestroyed())window.destroy();
    await profile.clearStorageData();
    await profile.clearCache();
    await local('/api/academic/browser-session?scope=all',undefined,'DELETE');
    imported=false;
    return {ok:true,message:'学校窗口和本次教务登录已清除'};
  }
  async function open(target){
    const url=schoolTargets[target]||target;
    if(!isSchoolURL(url))throw Error('不支持的学校页面');
    if(!window||window.isDestroyed()){
      window=new BrowserWindow({width:1100,height:820,minWidth:420,minHeight:520,title:'学校官方页面 · szuDesktop',
        webPreferences:{session:profile,contextIsolation:true,nodeIntegration:false,sandbox:true}});
      const win=window,wc=win.webContents;
      // The school keeps its own HTML/CSP and handles CAPTCHA, MFA and booking.
      wc.on('will-navigate',(event,url)=>{if(!isSchoolURL(url))event.preventDefault();});
      wc.on('will-redirect',(event,url)=>{if(!isSchoolURL(url))event.preventDefault();});
      wc.setWindowOpenHandler(({url})=>{if(isSchoolURL(url))void open(url).catch(showError);return {action:'deny'};});
      wc.on('page-title-updated',event=>event.preventDefault());
      wc.on('did-navigate',(_event,url)=>win.setTitle(new URL(url).hostname+' · 学校官方页面'));
      win.on('closed',()=>{if(window===win)window=null;});
      win.setMenu(Menu.buildFromTemplate([{label:'学校页面',submenu:[
        {label:'返回',click:()=>{if(wc.navigationHistory.canGoBack())wc.navigationHistory.goBack();}},
        {label:'前进',click:()=>{if(wc.navigationHistory.canGoForward())wc.navigationHistory.goForward();}},
        {label:'刷新',accelerator:'CmdOrCtrl+R',click:()=>wc.reload()},
        {label:'在系统浏览器打开',click:()=>{const url=wc.getURL();if(isSchoolURL(url))void shell.openExternal(url).catch(showError);}},
        {type:'separator'},
        {label:'清除本次学校登录',click:()=>void clear().catch(showError)},
        {label:'关闭学校窗口',click:()=>win.close()},
      ]}]));
    }
    window.show();window.focus();
    if(window.webContents.getURL()!==url)void window.loadURL(url).catch(showError);
    return {ok:true};
  }
  function showError(){dialog.showErrorBox('学校页面暂时无法打开','请检查校园网或 WebVPN；也可通过学校窗口菜单在系统浏览器中打开。');}
  async function sync(business){
    if(!Object.hasOwn(schoolTargets,business)||business==='booking')throw Error('请选择课表或成绩业务');
    const cookies=academicCookies(await profile.cookies.get({}));
    if(!cookies.length)throw Error('请先在应用内的学校页面登录；系统浏览器的登录状态不会自动共享');
    imported=true;
    return local('/api/academic/browser-session',{business,cookies});
  }
  function destroy(){if(window&&!window.isDestroyed())window.destroy();}
  async function shutdown(){destroy();if(imported)await local('/api/academic/browser-session',undefined,'DELETE');}
  return {open,sync,clear,destroy,shutdown};
}

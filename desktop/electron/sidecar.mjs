import {spawn, execFile} from 'node:child_process';
import {parseListenUrl} from './listen-url.mjs';

async function waitForUrl(child, timeoutMs){
  return new Promise((resolve,reject)=>{
    let buf='';const timer=setTimeout(()=>reject(new Error('sidecar 启动超时（没打印监听地址）')),timeoutMs);
    child.stdout.setEncoding('utf8');
    child.stdout.on('data',(c)=>{buf+=c;const url=parseListenUrl(buf);if(url){clearTimeout(timer);resolve(url);}});
    child.on('exit',(code)=>{clearTimeout(timer);reject(new Error('sidecar 提前退出，码 '+code));});
  });
}

async function waitHealthy(baseUrl, readyPath, timeoutMs){
  const deadline=Date.now()+timeoutMs;
  while(Date.now()<deadline){
    try{const r=await fetch(baseUrl+readyPath);if(r.ok)return;}catch{}
    await new Promise(r=>setTimeout(r,150));
  }
  throw new Error('sidecar 健康探测超时: '+baseUrl+readyPath);
}

export async function startSidecar({command,args=[],env,cwd,readyPath='/api/status',readyTimeoutMs=15000,healthTimeoutMs=8000}){
  const child=spawn(command,args,{env:env||process.env,cwd,stdio:['ignore','pipe','inherit']});
  const baseUrl=await waitForUrl(child,readyTimeoutMs);
  await waitHealthy(baseUrl,readyPath,healthTimeoutMs);
  return {baseUrl,child,stop:()=>stopSidecar(child)};
}

export function stopSidecar(child){
  return new Promise((resolve)=>{
    if(!child||child.exitCode!=null||child.killed){resolve();return;}
    const done=()=>resolve();
    child.once('exit',done);
    try{child.kill('SIGKILL');}catch{}
    if(process.platform==='win32'&&child.pid){
      // Windows 下子进程不随父进程自动结束，杀整棵进程树兜底
      execFile('taskkill',['/pid',String(child.pid),'/T','/F'],()=>{});
    }
    setTimeout(done,3000);
  });
}

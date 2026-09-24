import {spawn, execFile} from 'node:child_process';
import {setTimeout as delay} from 'node:timers/promises';
import {parseSidecarEndpoint} from './listen-url.mjs';

const hasExited=child=>child.exitCode!==null||child.signalCode!==null;

function waitForUrl(child, timeoutMs){
  return new Promise((resolve,reject)=>{
    let buf='';
    const cleanup=()=>{
      clearTimeout(timer);
      child.stdout.removeListener('data',onData);
      child.removeListener('error',onError);
      child.removeListener('close',onClose);
      // Keep draining logs without retaining their contents after discovery.
      child.stdout.resume();
    };
    const finish=(err,endpoint)=>{cleanup();if(err)reject(err);else resolve(endpoint);};
    const onError=err=>finish(new Error('sidecar 启动失败: '+err.message,{cause:err}));
    const onClose=code=>finish(new Error('sidecar 提前退出，码 '+code));
    const onData=chunk=>{
      buf+=chunk;
      const endpoint=parseSidecarEndpoint(buf);
      if(endpoint){finish(null,endpoint);return;}
      const lastNewline=buf.lastIndexOf('\n');
      if(lastNewline>=0)buf=buf.slice(lastNewline+1);
    };
    const timer=setTimeout(()=>finish(new Error('sidecar 启动超时（没打印监听地址）')),timeoutMs);
    child.stdout.setEncoding('utf8');
    child.stdout.on('data',onData);
    child.once('error',onError);
    // close follows stdout drain, so a short-lived reuse launcher is parsed first.
    child.once('close',onClose);
  });
}

function waitForExit(child,timeoutMs){
  if(hasExited(child))return Promise.resolve(true);
  return new Promise(resolve=>{
    const finish=exited=>{clearTimeout(timer);child.removeListener('exit',onExit);resolve(exited);};
    const onExit=()=>finish(true);
    const timer=setTimeout(()=>finish(false),timeoutMs);
    child.once('exit',onExit);
  });
}

async function waitHealthy(baseUrl, readyPath, timeoutMs, child){
  const controller=new AbortController();
  const timeoutError=new Error('sidecar 健康探测超时: '+baseUrl+readyPath);
  const timer=setTimeout(()=>controller.abort(timeoutError),timeoutMs);
  const onExit=code=>controller.abort(new Error('sidecar 提前退出，码 '+code));
  if(child){child.once('exit',onExit);if(hasExited(child))onExit(child.exitCode);}
  try{
    while(!controller.signal.aborted){
      try{
        const response=await fetch(baseUrl+readyPath,{signal:controller.signal});
        response.body?.cancel().catch(()=>{});
        if(response.ok)return;
      }catch(err){if(controller.signal.aborted)throw controller.signal.reason;}
      try{await delay(150,undefined,{signal:controller.signal});}catch{throw controller.signal.reason;}
    }
    throw controller.signal.reason;
  }finally{
    clearTimeout(timer);
    child?.removeListener('exit',onExit);
  }
}

export async function startSidecar({command,args=[],env,cwd,readyPath='/api/status',readyTimeoutMs=15000,healthTimeoutMs=8000}){
  const child=spawn(command,args,{env:env||process.env,cwd,stdio:['ignore','pipe','inherit'],windowsHide:true});
  let endpoint;
  try{
    endpoint=await waitForUrl(child,readyTimeoutMs);
    if(!endpoint.owned){
      if(!await waitForExit(child,readyTimeoutMs))throw new Error('sidecar 复用启动器退出超时');
      if(child.exitCode!==0)throw new Error('sidecar 提前退出，码 '+child.exitCode);
    }
    await waitHealthy(endpoint.baseUrl,readyPath,healthTimeoutMs,endpoint.owned?child:null);
  }catch(err){
    await stopSidecar(child);
    throw err;
  }
  let stopping;
  return {...endpoint,child,stop:()=>{
    if(!stopping)stopping=endpoint.owned?shutdownOwned(child,endpoint.baseUrl):Promise.resolve();
    return stopping;
  }};
}

async function shutdownOwned(child,baseUrl){
  if(hasExited(child))return;
  let accepted=false;
  try{
    const response=await fetch(baseUrl+'/api/shutdown',{
      method:'POST',headers:{'content-type':'application/json'},body:'{}',signal:AbortSignal.timeout(1000),
    });
    accepted=response.ok;
    response.body?.cancel().catch(()=>{});
  }catch{}
  if(accepted&&await waitForExit(child,2000))return;
  await stopSidecar(child);
}

const pendingStops=new WeakMap();
export function stopSidecar(child){
  if(!child)return Promise.resolve();
  if(pendingStops.has(child))return pendingStops.get(child);
  if(!child.pid||hasExited(child))return Promise.resolve();
  const stopping=(async()=>{
    if(process.platform==='win32'){
      // Kill the tree while its root is still alive. Killing the parent first
      // destroys the relationship taskkill needs to find its descendants.
      await new Promise((resolve,reject)=>execFile('taskkill',['/pid',String(child.pid),'/T','/F'],{windowsHide:true,timeout:3000},err=>{
        if(err&&!hasExited(child))reject(new Error('sidecar 进程树清理失败',{cause:err}));else resolve();
      }));
    }else{
      child.kill('SIGKILL');
    }
    if(!await waitForExit(child,1000))throw new Error('sidecar 退出超时');
  })();
  pendingStops.set(child,stopping);
  return stopping;
}

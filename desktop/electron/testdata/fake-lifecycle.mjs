import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import {spawn} from 'node:child_process';
const [mode,value]=process.argv.slice(2);
if(mode==='reuse'||mode==='reuse-failed'){
  process.stdout.write(`szuDesktop 已复用: ${value}\n`,()=>process.exit(mode==='reuse'?0:1));
}else{
  if(value)fs.writeFileSync(path.join(value,'parent.pid'),String(process.pid));
  if(mode==='tree'){
    const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore',windowsHide:true});
    await new Promise((resolve,reject)=>{child.once('spawn',resolve);child.once('error',reject);});
    fs.writeFileSync(path.join(value,'descendant.pid'),String(child.pid));
  }
  const server=http.createServer((req,res)=>{
    if(mode==='hanging')return;
    if(mode==='offline'&&req.url==='/api/status')return;
    if(mode==='offline'&&req.url==='/api/shutdown'){res.statusCode=404;res.end();return;}
    if(mode==='hanging-body'&&req.url==='/api/health'){res.writeHead(200,{'content-type':'application/json'});res.write('{"ok":');return;}
    if(mode==='graceful'&&req.url==='/api/shutdown'&&req.method==='POST'){
      fs.writeFileSync(path.join(value,'graceful.marker'),'shutdown accepted');
      req.resume();
      req.on('end',()=>res.end('{}',()=>server.close(()=>process.exit(0))));
      return;
    }
    res.statusCode=mode==='unhealthy'?500:200;
    res.end(req.url==='/api/health'?JSON.stringify({ok:true,app:mode==='wrong-app'?'other-app':'szuDesktop',app_version:'test'}):'{}');
  });
  server.listen(0,'127.0.0.1',()=>{
    const port=String(server.address().port);
    if(mode==='chunked'){
      process.stdout.write(`szuDesktop 已启动: http://127.0.0.1:${port.slice(0,1)}`);
      setTimeout(()=>process.stdout.write(port.slice(1)+'\n'),100);
    }else process.stdout.write(`szuDesktop 已启动: http://127.0.0.1:${port}\n`);
  });
}

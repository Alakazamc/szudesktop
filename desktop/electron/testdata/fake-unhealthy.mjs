// 测试替身：永不健康的 sidecar（/api/status 恒 500），用来验证「启动失败时子进程被杀掉、不留孤儿」。
import http from 'node:http';
import fs from 'node:fs';

// 退出标记：POSIX 上被 SIGKILL 时 exit 钩子仍会跑，marker 落盘即证明子进程已终止。
// Windows 上 stopSidecar 走 TerminateProcess / taskkill /F，不执行任何用户代码，marker 不会写出来，
// 所以额外落一个 PID 文件，让测试用 process.kill(pid,0) 探测进程是否真的消失（见 check-sidecar.mjs）。
process.on('exit',()=>{
  const marker=process.env.FAKE_EXIT_MARKER;
  if(!marker)return;
  try{fs.writeFileSync(marker,`${process.pid}\n`);}catch{}
});

if(process.env.FAKE_PID_FILE){
  try{fs.writeFileSync(process.env.FAKE_PID_FILE,String(process.pid));}catch{}
}

const srv=http.createServer((req,res)=>{
  if(req.url==='/api/status'){res.writeHead(500,{'content-type':'application/json'});res.end('{"ok":false}');return;}
  res.writeHead(404);res.end();
});
srv.listen(0,'127.0.0.1',()=>{
  const port=srv.address().port;
  process.stdout.write(`szuDesktop 已启动: http://127.0.0.1:${port}\n`);
});

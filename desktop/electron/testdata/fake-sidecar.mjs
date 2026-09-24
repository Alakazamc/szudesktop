import http from 'node:http';
const srv=http.createServer((req,res)=>{
  if(req.url==='/api/status'){res.writeHead(200,{'content-type':'application/json'});res.end('{"ok":true}');return;}
  res.writeHead(404);res.end();
});
srv.listen(0,'127.0.0.1',()=>{
  const port=srv.address().port;
  process.stdout.write(`szuDesktop 已启动: http://127.0.0.1:${port}\n`);
});

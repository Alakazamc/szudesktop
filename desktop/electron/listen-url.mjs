// Only complete protocol lines can announce an endpoint. A partially written port
// must never be accepted, and unrelated URLs in logs are not discovery records.
export function parseSidecarEndpoint(text){
  const lines=String(text||'').split('\n');
  lines.pop();
  for(const raw of lines){
    const match=/^szuDesktop (已启动|已复用): http:\/\/(?:127\.0\.0\.1|localhost):(\d{1,5})\r?$/.exec(raw);
    if(!match)continue;
    const port=Number(match[2]);
    if(port<1||port>65535)continue;
    return {baseUrl:`http://127.0.0.1:${port}`,owned:match[1]==='已启动'};
  }
  return null;
}
export function parseListenUrl(text){return parseSidecarEndpoint(text)?.baseUrl??null;}

// 端口只有一个来源：sidecar 打印的 "szuDesktop 已启动: http://127.0.0.1:<port>"。
export function parseListenUrl(text){
  const m=/http:\/\/(127\.0\.0\.1|localhost):(\d+)/.exec(text||'');
  return m?`http://127.0.0.1:${m[2]}`:null;
}

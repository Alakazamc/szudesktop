export const schoolTargets=Object.freeze({
  undergrad:'https://ehall.szu.edu.cn/jwapp/sys/wdkb/*default/index.do',
  graduate:'https://ehall.szu.edu.cn/yjsxkapp/sys/xsxkapp/*default/index.do',
  'undergrad-scores':'https://ehall.szu.edu.cn/jwapp/sys/cjcx/*default/index.do',
  'graduate-scores':'https://ehall.szu.edu.cn/gsapp/sys/xscjglapp/*default/index.do',
  booking:'https://swzx.webvpn.szu.edu.cn/#/pages/booth/szu-booth-list',
});
const hosts=new Set(['ehall.szu.edu.cn','authserver.szu.edu.cn','webvpn.szu.edu.cn','authserver-443.webvpn.szu.edu.cn',
  'swzx.webvpn.szu.edu.cn','swzx.szu.edu.cn']);
export function isSchoolURL(raw){
  try{const u=new URL(raw);return u.protocol==='https:'&&!u.username&&!u.password&&(!u.port||u.port==='443')&&hosts.has(u.hostname);}
  catch{return false;}
}
export function academicCookies(cookies){
  return cookies.filter(c=>['ehall.szu.edu.cn','.ehall.szu.edu.cn','szu.edu.cn','.szu.edu.cn'].includes(c.domain))
    .map(({name,value,path})=>({name,value,path}));
}

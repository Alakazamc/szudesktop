import {pixelIcon} from './pixel.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
export function createSchoolWindowUI({onSessionChanged}){
 let message='',error=false,busy=false;
 const available=()=>typeof globalThis.szuDesktop?.openSchool==='function';
 function card(booking=false){
  if(!available())return '';
  if(booking)return `<section class="card span"><h2 class="icon-heading tone-success">${pixelIcon('i-key','heading-icon')}在应用内办理预约</h2><p>打开学校预约窗口，直接登录、选择时段并提交。预约结果和取消也在同一个学校页面查看。</p><div class="actions"><button class="primary" data-action="official-booking">打开学校预约窗口</button></div><small>学校可能要求 WebVPN 和二次验证，请在原页面完成；以学校显示的预约结果为准。</small></section>`;
  return `<section class="card span" id="official-account"><h2 class="icon-heading tone-info">${pixelIcon('i-key','heading-icon')}学校账号 · 登录后读取</h2><p>先选择业务，在学校原页面完成登录；回到这里读取本次登录，再查看下面的课表或成绩。</p><label for="official-business">要查询的业务</label><select id="official-business"><option value="undergrad">本科课表</option><option value="graduate">研究生课表</option><option value="undergrad-scores">本科成绩</option><option value="graduate-scores">研究生成绩</option></select><div class="actions"><button class="primary" data-action="official-open">1. 打开学校登录</button><button data-action="official-sync">2. 读取本次登录</button><button class="quiet" data-action="official-clear">清除学校登录</button></div><p id="official-status" class="notice${error?' error':''}" role="status">${esc(message||'账号权限由学校决定；登录只保留到退出应用，不保存密码。')}</p><small>验证码和二次验证由学校页面处理；不同业务权限分别核验。</small></section>`;
 }
 function paint(){const status=document.getElementById('official-status');if(status){status.textContent=message;status.classList.toggle('error',error)}for(const el of document.querySelectorAll('#official-account button,#official-account select'))el.disabled=busy}
 async function click(action){
  if(!action.startsWith('official-'))return false;
  if(busy)return true;
  busy=true;error=false;message='正在处理…';paint();
  try{
   const target=document.getElementById('official-business')?.value||'undergrad';
   if(action==='official-open'||action==='official-booking'){
    await globalThis.szuDesktop.openSchool(action==='official-booking'?'booking':target);
    message='学校窗口已打开。登录完成后，返回这里点击「读取本次登录」。';
   }else if(action==='official-sync'){
    try{const result=await globalThis.szuDesktop.syncSchool(target);message=result.message}
    finally{await onSessionChanged()}
   }else if(action==='official-clear'){
    const result=await globalThis.szuDesktop.clearSchool();message=result.message;await onSessionChanged();
   }
  }catch(e){error=true;message=e.message.replace(/^Error invoking remote method '[^']+': Error: /,'')}
  finally{busy=false;paint()}
  return true;
 }
 return {card,click};
}

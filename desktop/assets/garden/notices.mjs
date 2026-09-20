import {pixelIcon} from './pixel.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const directory='https://www.szu.edu.cn/yxjg/xbxy.htm';
const link=(url,text)=>`<a class="button quiet" href="${esc(url)}" target="_blank" rel="noopener noreferrer">${text} ↗</a>`;
export function createNoticesUI({api}){
 let sources=[],source='undergrad',catalogLoading=false,catalogError='',feed=null,error='',loading=false,requestVersion=0;
 const current=()=>sources.find(s=>s.id===source);
 function feedHTML(){
  const chosen=current();
  if(!chosen)return '';
  if(!chosen.readable)return `<p class="notice" role="status">${esc(chosen.note||'暂未接入应用内读取，请查看学院官网。')}</p>`;
  if(loading)return `<p role="status">正在读取${esc(chosen.name)}的公开公告…</p>`;
  if(error)return `<p role="status" class="notice error">${esc(error)}</p>`;
  if(!feed)return '<p class="muted">选择学院后自动读取；也可以点击「读取公告」或直接查看原页。</p>';
  const sorted=[...feed.items].sort((a,b)=>b.date.localeCompare(a.date));
  return `<p class="notice">${esc(feed.source)} · ${feed.stale?'上次读取的内容':'读取于 '+esc(new Date(feed.fetched_at).toLocaleString('zh-CN',{month:'long',day:'numeric',hour:'2-digit',minute:'2-digit'}))}${feed.message?' · '+esc(feed.message):''}</p><ul class="campus-notices">${sorted.slice(0,12).map(x=>`<li><time>${esc(x.date)}</time><a href="${esc(x.url)}" target="_blank" rel="noopener noreferrer">${esc(x.title)}</a></li>`).join('')}</ul>`;
 }
 function content(){
  const chosen=current();
  const options=[['university','全校通知'],['college','学院与学部']].map(([group,label])=>`<optgroup label="${label}">${sources.filter(s=>s.group===group).map(s=>`<option value="${esc(s.id)}" ${s.id===source?'selected':''}>${esc(s.name)}${s.readable?'':' · 官网查看'}</option>`).join('')}</optgroup>`).join('');
  return `<div class="card-head"><h2 class="icon-heading tone-warning">${pixelIcon('i-bell','heading-icon')}学校公告</h2><span class="badge" data-tone="info">按学院查看</span></div><p class="muted">按发布单位查看公开通知，不包含需要登录的校内公告。</p>
  ${catalogLoading?'<p role="status">正在准备学院列表…</p>':catalogError?`<p role="status" class="notice error">${esc(catalogError)}</p><div class="actions"><button data-action="notice-sources">重新加载学院列表</button>${link(directory,'学校院系目录')}</div>`:chosen?`<div class="actions"><div class="notice-source"><label for="feed-source">学院／部门</label><select id="feed-source">${options}</select></div>${chosen.readable?`<button data-action="notice-read" ${loading?'disabled':''}>${pixelIcon('i-bell')}读取公告</button>`:''}${link(chosen.url,chosen.readable?'查看原页':'打开学院官网')}</div>${chosen.readable&&chosen.note?`<p class="muted">当前栏目：${esc(chosen.note)}</p>`:''}<div id="campus-feed-content" aria-live="polite">${feedHTML()}</div>${chosen.readable?'<small>当前栏目首页最多展示 12 条，10 分钟内复用。仅收录可核对日期的本站公告，更多通知请查看原页。</small>':''}`:'<p role="status">正在准备学院列表…</p>'}`;
 }
 const card=()=>`<section class="card campus-feed" id="notice-panel">${content()}</section>`;
 const paint=()=>{const el=document.getElementById('notice-panel');if(el)el.innerHTML=content()};
 async function load(){
  if(catalogLoading||sources.length)return;
  catalogLoading=true;catalogError='';paint();
  try{const result=await api('/api/campus/notice-sources');sources=result.sources;if(!current())source=sources[0]?.id||'';if(!sources.length)catalogError='暂时没有可用的公告来源，请查看学校院系目录。'}
  catch(e){catalogError=e.message}
  finally{catalogLoading=false;paint()}
 }
 async function read(){
  if(!current()?.readable)return;
  const selected=source,version=++requestVersion;
  feed=null;error='';loading=true;paint();
  try{const result=await api('/api/campus/notices?source='+encodeURIComponent(selected));if(version===requestVersion)feed=result}
  catch(e){if(version===requestVersion)error=e.message}
  finally{if(version===requestVersion){loading=false;paint()}}
 }
 async function click(action){if(action==='notice-read'){await read();return true}if(action==='notice-sources'){await load();return true}return false}
 async function change(e){
  if(e.target.id!=='feed-source')return false;
  if(!sources.some(s=>s.id===e.target.value))return true;
  source=e.target.value;requestVersion++;feed=null;error='';loading=false;paint();
  if(current().readable)await read();
  return true;
 }
 return {card,load,click,change};
}

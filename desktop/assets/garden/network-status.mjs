import {pixelIcon} from './pixel.mjs';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

export function networkBadge(status) {
  return status ? (status.internet_ok ? '外网可用' : '外网不可用') : '状态未确认';
}

export const networkTone=status=>!status?'muted':status.internet_ok?'success':'error';
export const networkBadgeHTML=status=>pixelIcon(!status?'i-compass':status.internet_ok?'i-signal':'i-disconnect')+`<span>${networkBadge(status)}</span>`;

export function networkSummaryHTML(status,error='') {
  const row=(tone,icon,title,detail)=>`<div class="network-line" data-tone="${tone}">${pixelIcon(icon)}<div><strong>${title}</strong><span>${esc(detail)}</span></div></div>`;
  if(!status)return row('muted','i-compass','状态未确认',error||'网络状态尚未确认，请刷新后重试。');
  const connection=row(networkTone(status),status.internet_ok?'i-signal':'i-disconnect',networkBadge(status),status.zone_label);
  if(status.online_known)return connection+row(status.online?'success':'warning','i-shield','校园认证',status.online?'当前网络出口已在线。':'门户未检测到在线会话。');
  return connection+row('warning','i-compass','校园认证待确认',status.online_error?'暂未查明：'+status.online_error:'当前网络未确认校园认证状态。');
}

export function networkSummary(status) {
  if (!status) return '网络状态尚未确认，请刷新后重试。';
  const connection = `${networkBadge(status)} · ${status.zone_label}`;
  if (status.online_known) return connection + (status.online
    ? '。校园认证：当前网络出口已在线。'
    : '。校园认证：门户未检测到在线会话。');
  return connection + (status.online_error
    ? '。校园认证状态暂未查明：' + status.online_error
    : '。当前网络未确认校园认证状态。');
}

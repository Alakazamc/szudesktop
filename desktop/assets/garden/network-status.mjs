export function networkBadge(status) {
  return status ? (status.internet_ok ? '外网可用' : '外网不可用') : '状态未确认';
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

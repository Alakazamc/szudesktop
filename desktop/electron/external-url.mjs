// 外链协议白名单：只放行带 `//` 的 http:/https:，其余（file:、search-ms:、
// ms-msdt:、javascript:、data:、空串、相对路径、协议相对 //host）一律拒绝，
// 杜绝把任意 URI 交给系统处理器。
export function isSafeExternalUrl(url){ return /^https?:\/\//i.test(String(url||'')); }

// 深大某学院「琴房管理预约系统」的只读接入。
//
// 立场（和场馆预约一致）：这里只查询、不代提交。预约 / 取消 / 开门一律去原系统页面做，
// 本模块不提供任何写操作按钮。卡号密码不保存：只在登录那一次发给本机服务，换回的
// token 存在服务端内存里，重启即失效，所以页面登录态也随重启重置。
const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

// 原系统前端地址，「去原系统预约」跳这里。
export const PIANO_SITE = 'http://192.168.197.131:8080/#/loginPage';

const dash = s => (s ? esc(s) : '<span class="muted">—</span>');

export function pianoLoginHTML() {
  return `<div class="piano-login">
    <label class="muted" for="piano-card">校园卡号</label>
    <input id="piano-card" type="text" autocomplete="off" placeholder="卡号">
    <label class="muted" for="piano-pwd">密码</label>
    <input id="piano-pwd" type="password" autocomplete="off" placeholder="琴房系统密码">
    <div class="actions"><button data-action="piano-login">登录琴房系统</button></div>
    <small class="muted">卡号和密码只在这一次登录里使用，不会保存到本机；登录成功后也只在内存里留一个临时凭证。此学院系统目前使用校内 HTTP，请在可信校园网中使用琴房专用密码。</small>
  </div>`;
}

export function pianoRoomsHTML(rooms) {
  if (!rooms || rooms.length === 0) return '<p class="notice">没有拿到琴房列表。可能是当前网络到不了琴房系统，或列表为空。</p>';
  return `<div class="table-wrap"><table class="piano-table"><thead><tr><th>琴房</th><th>管理人</th><th>说明</th></tr></thead><tbody>${rooms.map(r => `<tr><td>${dash(r.name)}</td><td>${dash(r.manager)}</td><td>${dash(r.desc)}</td></tr>`).join('')}</tbody></table></div>`;
}

export function pianoMyHTML(list) {
  if (!list || list.length === 0) return '<p class="notice">暂时没有你的预约记录。</p>';
  return `<div class="table-wrap"><table class="piano-table"><thead><tr><th>琴房</th><th>时间段</th><th>状态</th></tr></thead><tbody>${list.map(r => `<tr><td>${dash(r.room)}</td><td>${dash(r.time)}</td><td>${dash(r.sign)}</td></tr>`).join('')}</tbody></table></div>`;
}

export function createPianoUI({ api, toast, render }) {
  let loggedIn = false;
  let rooms=null,reservations=null,page=1,count=0,generation=0;

  function section() {
    const body = loggedIn
      ? `<div class="actions">
           <button data-action="piano-rooms">刷新琴房列表</button>
           <button data-action="piano-my">我的预约</button>
           <button data-action="piano-logout">退出琴房登录</button>
           <a class="btn quiet" href="${PIANO_SITE}" target="_blank" rel="noopener noreferrer">去原系统预约 ↗</a>
         </div>
         <div id="piano-rooms">${roomContent()}</div>
         <div id="piano-my">${reservations===null?'':pianoMyHTML(reservations)}</div>
         <small class="muted">查询结果仅供参考；预约、取消、开门请到原系统页面操作，并以那里为准。</small>`
      : pianoLoginHTML();
    return `<section class="card span"><div class="card-head"><h2>学院琴房 · 只读查询</h2><span class="badge">接入测试 · 待账号验收</span></div><p class="muted">与社区共享琴房分开，需要该学院系统授予的账号权限。</p>${body}</section>`;
  }
  function roomContent(){return rooms===null?'':pianoRoomsHTML(rooms)+`<div class="actions"><button data-action="piano-prev" ${page<=1?'disabled':''}>上一页</button><span>第 ${page} 页 · 共 ${count} 间</span><button data-action="piano-next" ${page*20>=count?'disabled':''}>下一页</button></div>`}
  function reset(){generation++;rooms=null;reservations=null;page=1;count=0}
  async function load(){try{loggedIn=!!(await api('/api/piano/status')).logged_in}catch{loggedIn=false}}

  async function click(a, b) {
    if (a === 'piano-login') {
      const card = document.querySelector('#piano-card');
      const pwd = document.querySelector('#piano-pwd');
      if (!card || !pwd) return true;
      reset();loggedIn=false;
      try{await api('/api/piano/login', { cardNo: card.value, pwd: pwd.value });}
      finally{pwd.value='';}
      loggedIn = true;
      toast('琴房系统登录成功');
      render();
      return true;
    }
    if (a === 'piano-logout') {
      await api('/api/piano/logout', {});
      loggedIn = false;
      reset();
      toast('已退出琴房登录');
      render();
      return true;
    }
    if (['piano-rooms','piano-next','piano-prev'].includes(a)) {
      const turn=generation,requested=a==='piano-next'?page+1:a==='piano-prev'?Math.max(1,page-1):page;
      let d;
      try { d = await api('/api/piano/rooms?pageIndex='+requested+'&pageSize=20'); }
      catch (e) { if(turn!==generation)return true;if (e && e.code === 401) { loggedIn = false;reset(); render(); } throw e; }
      if(turn!==generation)return true;
      page=requested;rooms=d.rooms;count=d.count;
      const el = document.querySelector('#piano-rooms');
      if (el) el.innerHTML = roomContent();
      return true;
    }
    if (a === 'piano-my') {
      const turn=generation;
      let d;
      try { d = await api('/api/piano/my'); }
      catch (e) { if(turn!==generation)return true;if (e && e.code === 401) { loggedIn = false;reset(); render(); } throw e; }
      if(turn!==generation)return true;
      reservations=d.list;
      const el = document.querySelector('#piano-my');
      if (el) el.innerHTML = pianoMyHTML(d.list);
      return true;
    }
    return false;
  }

  return { section, click, load, get loggedIn() { return loggedIn; } };
}

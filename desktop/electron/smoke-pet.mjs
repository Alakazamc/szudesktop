// Invoked only by the isolated application smoke mode, including the final NSIS package.
import assert from 'node:assert/strict';
import {writeFileSync} from 'node:fs';
import path from 'node:path';
import {readPetSettings} from './pet-settings.mjs';
import {petWindowBounds} from './pet-policy.mjs';

async function until(read, message) {
  const end=Date.now()+6000;
  while(Date.now()<end){if(await read())return;await new Promise(r=>setTimeout(r,50));}
  throw Error(message);
}

export async function checkPetRuntime({mainWin,petWin,tray,getMenu,getPetMenu,screen,initialScale,userData,evidenceDir,baseUrl}){
  const trace=stage=>writeFileSync(path.join(evidenceDir,'pet-progress.json'),JSON.stringify({stage,bounds:petWin?.getBounds(),visible:petWin?.isVisible()}));
  trace('start');
  assert.ok(petWin&&!petWin.isDestroyed()&&petWin.isVisible(),'pet window exists');
  assert.ok(tray&&!tray.isDestroyed(),'packaged tray icon loads');
  assert.ok(petWin.isAlwaysOnTop(),'pet stays on top');
  const main=source=>mainWin.webContents.executeJavaScript(source);
  const pet=source=>petWin.webContents.executeJavaScript(source);
  const menu=label=>getMenu().items.find(item=>item.label===label);
  const sizeItems=()=>menu('宠物大小').submenu.items;
  await main("document.querySelector('#guide[open] button')?.click()");
  await until(()=>main("!document.querySelector('#guide[open]')"),'first-run guide did not close');
  await until(()=>pet("Boolean(window.szuPet && document.querySelector('#pet-use')?.getAttribute('href'))"),'pet preload/render not ready');
  assert.equal(await main('window.szuDesktop.petScale()'),initialScale);
  assert.equal(readPetSettings(userData).scale,initialScale,'scale loaded from previous launch');
  const stored=readPetSettings(userData);
  if(stored.position){
    const bounds=petWin.getBounds(),area=screen.getDisplayMatching(bounds).workArea;
    assert.deepEqual(bounds,petWindowBounds(area,initialScale,stored.position),'saved position restored');
  }

  mainWin.close();
  trace('main-hidden');
  assert.ok(!mainWin.isDestroyed()&&!mainWin.isVisible(),'close hides main without destroying it');
  const target=await pet("(()=>{const r=document.querySelector('#pet').getBoundingClientRect();return {x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)}})()");
  const beforeDrag=petWin.getBounds(),dragArea=screen.getDisplayMatching(beforeDrag).workArea;
  const globalPoint={globalX:beforeDrag.x+target.x,globalY:beforeDrag.y+target.y};
  const dragPoint={globalX:globalPoint.globalX-48,globalY:globalPoint.globalY-36};
  petWin.webContents.sendInputEvent({type:'mouseDown',...target,...globalPoint,button:'left',clickCount:1});
  petWin.webContents.sendInputEvent({type:'mouseMove',x:target.x-48,y:target.y-36,...dragPoint,button:'left',modifiers:['leftButtonDown']});
  petWin.webContents.sendInputEvent({type:'mouseUp',x:target.x-48,y:target.y-36,...dragPoint,button:'left',clickCount:1});
  const dragged=petWindowBounds(dragArea,initialScale,{x:beforeDrag.x-48,y:beforeDrag.y-36});
  await until(()=>{const p=readPetSettings(userData).position;return p?.x===dragged.x&&p?.y===dragged.y;},'drag did not persist the pointer movement');
  assert.deepEqual(petWin.getBounds(),dragged,'drag follows both pointer axes');
  assert.equal(getPetMenu(),null,'drag must not open the click menu');
  trace('dragged');
  petWin.webContents.sendInputEvent({type:'mouseDown',...target,button:'left',clickCount:1});
  petWin.webContents.sendInputEvent({type:'mouseUp',...target,button:'left',clickCount:1});
  trace('clicked');
  await until(()=>Boolean(getPetMenu()?.getMenuItemById('feed')),'click did not open the pet menu');
  assert.equal(mainWin.isVisible(),false,'click opens a menu without opening main');
  getPetMenu().closePopup(petWin);
  trace('menu-closed');
  const game=async()=>{const r=await fetch(baseUrl+'/api/workspace');assert.ok(r.ok);return (await r.json()).data.game;};
  const active=g=>g.pets[g.active]||g.pets[0];
  for(let i=0;i<2;i++){
    trace('sleep-'+i);
    const before=active(await game()).sleeping;
    getPetMenu().getMenuItemById('sleep').click();
    await until(async()=>active(await game()).sleeping!==before,'pet sleep menu did not save');
    await until(()=>pet(`document.querySelector('#bubble-text').textContent.includes('${before?'醒来':'晚安'}')`),'saved action did not reach the pet bubble');
  }
  const beforeFeed=await game(),canFeed=!active(beforeFeed).sleeping&&active(beforeFeed).hunger<98&&beforeFeed.food>0;
  trace('feed');
  getPetMenu().getMenuItemById('feed').click();
  await until(()=>pet("/吃饱|唤醒|食物用完/.test(document.querySelector('#bubble-text').textContent)"),'feed result was not shown');
  assert.equal((await game()).food,beforeFeed.food-(canFeed?1:0),'food changes only after a valid meal');
  assert.equal(mainWin.isVisible(),false,'care works with the main window hidden');
  getPetMenu().getMenuItemById('farm').click();
  trace('farm');
  await until(()=>main("location.hash==='#garden' && Boolean(document.querySelector('#seed-choice'))"),'farm menu did not select the farm');
  getPetMenu().getMenuItemById('study').click();
  await until(()=>main("location.hash==='#study'"),'study menu did not navigate');
  getPetMenu().getMenuItemById('home').click();
  await until(()=>main("location.hash==='#home'"),'main menu did not restore home');
  await until(()=>mainWin.isVisible(),'pet menu cannot restore main window');
  await until(async()=>['idle','happy','sad','sleep'].includes(await pet("document.querySelector('#pet').dataset.action")),'one-shot action never returns to base');
  mainWin.close();
  menu('打开主窗口').click();
  assert.ok(mainWin.isVisible(),'tray opens main');
  menu('隐藏宠物').click();
  assert.equal(petWin.isVisible(),false);
  menu('显示宠物').click();
  assert.equal(petWin.isVisible(),true);

  for(const scale of [0.6,1,1.5]){
    sizeItems().find(item=>item.label.includes(`${scale*100}%`)).click();
    assert.equal(await main('window.szuDesktop.petScale()'),scale);
    assert.equal(sizeItems().filter(item=>item.checked).length,1);
  }
  await main("document.querySelector('[data-action=\"navigate\"][data-page=\"settings\"]').click()");
  await until(()=>main("document.querySelector('#pet-scale')?.value==='1.5'"),'settings slider did not bind after rendering');
  for(const scale of [0.4,2,1.7]){
    await main(`(()=>{const input=document.querySelector('#pet-scale');input.value='${scale}';input.dispatchEvent(new Event('change',{bubbles:true}));})()`);
    await until(async()=>await main('window.szuDesktop.petScale()')===scale,'settings scale did not reach main process');
    await until(()=>pet(`Number(getComputedStyle(document.documentElement).getPropertyValue('--pet-scale'))===${scale}`),'pet CSS scale did not update');
    const bounds=petWin.getBounds(),area=screen.getDisplayMatching(bounds).workArea;
    const settings=readPetSettings(userData);
    assert.deepEqual(bounds,petWindowBounds(area,scale,settings.position),'pet bounds follow display work area and saved position');
    assert.equal(sizeItems().filter(item=>item.checked).length,0,'custom scale has no false preset check');
    assert.equal(readPetSettings(userData).scale,scale,'scale persisted');
    petWin.webContents.send('pet:say','嗨，今天也一起加油。');
    await until(()=>pet("document.querySelector('#bubble').classList.contains('show') && !document.querySelector('#bubble').getAnimations().some(a=>a.playState==='running')"),'pet bubble did not settle');
    await pet('document.fonts.ready.then(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))))');
    writeFileSync(path.join(evidenceDir,`pet-${Math.round(scale*100)}.png`),(await petWin.webContents.capturePage()).toPNG());
  }
  writeFileSync(path.join(evidenceDir,'pet-settings.png'),(await mainWin.webContents.capturePage()).toPNG());
  await main("document.querySelector('[data-action=\"navigate\"][data-page=\"study\"]').click()");
  assert.ok(await main("Boolean(document.querySelector('#official-account [data-action=\"official-open\"]'))"),'school login entry rendered');
  assert.equal(await main("Boolean(document.querySelector('#session-cookie'))"),false,'installed UI does not ask for cookies');
  await main('document.fonts.ready.then(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))))');
  writeFileSync(path.join(evidenceDir,'school-account.png'),(await mainWin.webContents.capturePage()).toPNG());
  await main("document.querySelector('[data-action=\"navigate\"][data-page=\"home\"]').click()");
  return {rendered:true,tray:true,closeAndReopen:true,hideAndShow:true,actionsReturnToBase:true,
    initialScale,finalScale:1.7,settingsAndPresets:true,petMenu:true,hiddenCare:true,feedUsesInventory:true,menuNavigation:true,drag:true,positionPersistence:true,displayCount:screen.getAllDisplays().length};
}

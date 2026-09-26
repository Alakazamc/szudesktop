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

export async function checkPetRuntime({mainWin,petWin,tray,getMenu,screen,initialScale,userData,evidenceDir}){
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

  mainWin.close();
  assert.ok(!mainWin.isDestroyed()&&!mainWin.isVisible(),'close hides main without destroying it');
  await pet("document.querySelector('#pet').dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true}))");
  await until(()=>mainWin.isVisible(),'pet cannot restore main window');
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
    assert.deepEqual(bounds,petWindowBounds(area,scale),'pet bounds follow display work area');
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
    initialScale,finalScale:1.7,settingsAndPresets:true,displayCount:screen.getAllDisplays().length};
}

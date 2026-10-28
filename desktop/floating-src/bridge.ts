// MacroLeprechaun host adapter for Kando's MIT-licensed renderer.
import {SoundTheme} from './menu-renderer/sound-theme';
import {SettingsButton} from './menu-renderer/settings-button';
import {Menu} from './menu-renderer/menu';
import {MenuTheme} from './menu-renderer/menu-theme';
import {CenterText} from './menu-renderer/center-text';
import {MenuInteractionType} from './common';
import defaults from './defaults.json';
import JSON5 from 'json5';

(window as any).FloatingThemeParse=(raw:string)=>JSON5.parse(raw);
const query=new URLSearchParams(location.search),preview=query.has('preview'),token=query.get('admin')||'',host=query.get('host')||'';
let sound:SoundTheme,settingsButton:SettingsButton;
if(preview)document.getElementById('settings-button')!.style.display='none';
const container=document.getElementById('kando-menu')!,theme=new MenuTheme(),text=new CenterText(container);
let menu:Menu,panel:any,settings:any,selected:string[]=[],lastModifiers={ctrlKey:false,shiftKey:false},request=0;
function send(value:any){if(preview)parent.postMessage({floating:true,...value},location.origin);else (window as any).external?.invoke?.(JSON.stringify(value))}
function label(value:string,variables:any){return String(value||'').replace(/\$\{([A-Za-z0-9_-]+)\}/g,(all,key)=>Object.prototype.hasOwnProperty.call(variables,key)?String(variables[key]):all)}
async function api(path:string){const r=await fetch('/api/'+path,{headers:token?{Authorization:'Bearer '+token}:{}});if(!r.ok)throw Error(await r.text());return r.json()}
function parseTheme(entry:any){const d=JSON5.parse(entry.description);if(d.engineVersion!==1||!Array.isArray(d.layers)||d.layers.length>32)throw Error('Unsupported theme engine');for(const layer of d.layers)if(!/^[\w-]+$/.test(layer.class))throw Error('Invalid layer class');return{...d,id:entry.id,directory:'/floating-effects',drawCenterText:d.drawCenterText!==false,drawWedgeSeparators:!!d.drawWedgeSeparators}}
async function show(payload:any){const generation=++request;panel=payload.panel;document.documentElement.style.colorScheme=payload.colorScheme||'dark';const config=panel.floatingStyle||{},entries=payload.themes,entry=entries.find((e:any)=>e.id===(config.theme||'default'))||entries.find((e:any)=>e.id==='default');if(!entry)throw Error('No floating themes installed');const description=parseTheme(entry);settings={...defaults,...config.options};settings.zoomFactor=Math.max(.5,Math.min(3,Number(settings.zoomFactor)||1));if(preview){settings.keepInputFocus=true;settings.enableTurboMode=false;settings.enableMarkingMode=false;settings.enableGamepad=false;settings.windowsInkWorkaround=false}
 const colors={...description.colors,...config.colors};if(!panel.floatingStyle&&entry.id==='default'){const dark=entry.presets?.['Dark Night'];if(dark)Object.assign(colors,dark.colors)}
 if(!menu){menu=new Menu(container,theme,text,settings as any);menu.on('interaction',(type,path)=>{if(!preview)sound?.playSound(type);if(type===MenuInteractionType.eSelectButton){const b=panel.controls[path[0]];if(b){if(preview){send({type:'select',control:b.id,...lastModifiers});paintSelection()}else {menu.triggerInteraction(MenuInteractionType.eCloseMenu);send({type:'activate',panel:panel.id,control:b.id,fade:settings.fadeOutDuration,returnPointer:settings.returnPointerToMenuOpeningPosition})}}}else if(type===MenuInteractionType.eCloseMenu){if(!preview)send({type:'close',fade:settings.fadeOutDuration,returnPointer:settings.returnPointerToMenuOpeningPosition})}});menu.on('move-pointer',offset=>{if(!preview&&settings.warpMouse)send({type:'warp',x:offset.x*devicePixelRatio*settings.zoomFactor,y:offset.y*devicePixelRatio*settings.zoomFactor})})}
 if(!sound)sound=new SoundTheme();const soundEntry=entries.find((e:any)=>e.id===settings.soundTheme);sound.loadDescription(soundEntry?{...JSON5.parse(soundEntry.description),id:soundEntry.id,directory:'/floating-effects'}:{sounds:{}} as any);sound.setVolume(Math.min(1,Math.max(0,Number(settings.soundVolume))));
 if(!preview){if(!settingsButton){settingsButton=new SettingsButton(document.getElementById('settings-button') as HTMLButtonElement,settings);settingsButton.on('click',()=>send({type:'settings',panel:panel.id}))}settingsButton.updateSettings(settings);settingsButton.show()}
 menu.updateSettings(settings as any);theme.loadDescription(description);theme.setColors(colors);text.setDiameter(description.centerTextWrapWidth||95);text.setEnabled(description.drawCenterText);
 const stylesheet=document.getElementById('kando-menu-theme') as HTMLLinkElement;await new Promise(resolve=>{stylesheet.onload=resolve;stylesheet.onerror=resolve;setTimeout(resolve,1500)});if(generation!==request)return;
 const root:any={type:'root',name:panel.name,icon:'✦',iconTheme:'macro',children:panel.controls.map((b:any)=>({type:'button',name:label(b.label,payload.variables||{}),icon:b.icon||'',iconTheme:'macro',selectWorkflow:{quickSelectKey:b.quickSelectKey}}))};
 const zoom=settings.zoomFactor;container.style.zoom=String(zoom);const width=innerWidth/zoom,height=innerHeight/zoom;menu.show(root,{mousePosition:{x:preview?width/2:Number(query.get('cursorX'))/devicePixelRatio/zoom||width/2,y:preview?height/2:Number(query.get('cursorY'))/devicePixelRatio/zoom||height/2},windowSize:{x:width,y:height},zoomFactor:zoom,useFixedPosition:preview||!!settings.useFixedPosition,fixedMenuPosition:{x:settings.useFixedPosition?(settings.fixedPositionX??.5):.5,y:settings.useFixedPosition?(settings.fixedPositionY??.5):.5},anchoredMode:preview||!!settings.anchoredMode,hoverMode:!preview&&!!settings.hoverMode,systemIconsChanged:false});
 paintSelection();if(!preview&&settings.windowsInkWorkaround)await new Promise(resolve=>setTimeout(resolve,110));if(!preview)send({type:'ready',viewportWidth:innerWidth,viewportHeight:innerHeight,keepInputFocus:!!settings.keepInputFocus,hideOnFocusOut:!!settings.hideOnFocusOut,fade:settings.fadeOutDuration});
}
function paintSelection(){const [root]=menu?.getCurrentRequest()||[];if(!root)return;root.children.forEach((item:any,i:number)=>{const node=item.renderData?.nodeDiv;if(node){node.classList.toggle('macro-selected',selected.includes(panel.controls[i]?.id));node.dataset.controlId=panel.controls[i]?.id||''}})}
container.addEventListener('pointerdown',e=>{lastModifiers={ctrlKey:e.ctrlKey||e.metaKey,shiftKey:e.shiftKey}},true);
container.addEventListener('contextmenu',e=>{e.preventDefault();if(preview){const hovered=container.querySelector('.menu-node.child.hovered') as HTMLElement;if(hovered)send({type:'context',control:hovered.dataset.controlId,x:e.clientX,y:e.clientY})}},true);
window.addEventListener('message',e=>{if(e.source!==parent||e.origin!==location.origin)return;if(e.data?.type==='floating-show')show(e.data).catch(fail);if(e.data?.type==='floating-values'){const [root]=menu?.getCurrentRequest()||[];if(root){root.children.forEach((b:any,i:number)=>b.name=label(panel.controls[i].label,e.data.variables));(menu as any).redraw()}}if(e.data?.type==='floating-selection'){selected=e.data.ids||[];paintSelection()}});
window.addEventListener('resize',()=>{if(preview)send({type:'resize'})});
function fail(e:any){document.getElementById('error')!.textContent=String(e.message||e);send({type:'error',message:String(e.message||e)})}
if(preview){send({type:'ready'})}else{Promise.all([api('floating/state?host='+encodeURIComponent(host)),api('floating/themes')]).then(([data,themes])=>show({...data,themes})).catch(fail)}
(window as any).FloatingDefaults=defaults;

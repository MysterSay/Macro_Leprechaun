// MacroLeprechaun adapter. Icons are existing control symbols or local assets.
export class IconThemeRegistry {
 static instance=new IconThemeRegistry();static getInstance(){return this.instance}
 createIcon(theme:string,value:string){const node=document.createElement('div');node.className='icon-container';if(value.startsWith('/media/icon/')){const image=document.createElement('img');image.src=value;node.append(image)}else{const text=document.createElement('i');text.textContent=value;node.append(text)}return node}
}

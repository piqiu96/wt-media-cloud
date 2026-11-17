// Attached drawers share a page scroll lock; nested drawers release it last.
const owners = new Set();
let restore;
export function setOverlayScrollLock(owner, active) {
 if (active) {
  if (owners.has(owner) || typeof document === 'undefined' || typeof window === 'undefined') return;
  if (!owners.size) {
   const x=window.scrollX, y=window.scrollY;
   const records=[[document.documentElement,'overflow'],[document.body,'overflow'],[document.body,'width']].map(([element,property])=>({element,property,value:element.style.getPropertyValue(property),priority:element.style.getPropertyPriority(property)}));
   const gutter=Math.max(0,window.innerWidth-document.documentElement.clientWidth);
   document.documentElement.style.setProperty('overflow','hidden');
   document.body.style.setProperty('overflow','hidden');
   if(gutter)document.body.style.setProperty('width',`calc(100% - ${gutter}px)`);
   restore=()=>{
    for(const {element,property,value,priority}of records)if(value)element.style.setProperty(property,value,priority);else element.style.removeProperty(property);
    window.scrollTo(x,y);
   };
  }
  owners.add(owner);
 } else if (owners.delete(owner) && !owners.size) {
  const finish=restore;restore=null;finish?.();
 }
}

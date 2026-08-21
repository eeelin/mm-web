export type UnreadKind = 'messages' | 'diagnostics' | 'phone';
export type UnreadState = Record<UnreadKind, number>;

const cacheName = 'mmos-state-v1';
const statePath = '/__mmos_unread_state__';
export const emptyUnreadState = ():UnreadState => ({messages:0,diagnostics:0,phone:0});

export async function readUnreadState():Promise<UnreadState> {
  if (!('caches' in window)) return emptyUnreadState();
  try {
    const cache=await caches.open(cacheName); const response=await cache.match(new URL(statePath,location.origin));
    return response?{...emptyUnreadState(),...await response.json() as Partial<UnreadState>}:emptyUnreadState();
  } catch { return emptyUnreadState(); }
}

export async function clearUnread(kind:UnreadKind):Promise<void> {
  const state=await readUnreadState(); state[kind]=0;
  if ('caches' in window) {const cache=await caches.open(cacheName);await cache.put(new URL(statePath,location.origin),new Response(JSON.stringify(state),{headers:{'Content-Type':'application/json'}}))}
  const total=state.messages+state.diagnostics+state.phone;
  if (total>0&&'setAppBadge'in navigator) await navigator.setAppBadge(total).catch(()=>{});
  else if ('clearAppBadge'in navigator) await navigator.clearAppBadge().catch(()=>{});
  window.dispatchEvent(new CustomEvent('mmos-unread-change',{detail:state}));
}

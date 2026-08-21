self.addEventListener('install', event => {
  event.waitUntil(self.skipWaiting());
});

const unreadCache='mmos-state-v1';
const unreadURL=()=>new URL('/__mmos_unread_state__',self.location.origin);
const emptyUnread=()=>({messages:0,diagnostics:0,phone:0});
const kindForURL=value=>{const screen=new URL(value||'/?screen=messages',self.location.origin).searchParams.get('screen');return screen==='diagnostics'?'diagnostics':screen==='phone'?'phone':'messages'};
let unreadUpdate=Promise.resolve();
async function updateUnread(url) {
  const cache=await caches.open(unreadCache);const response=await cache.match(unreadURL());
  const state=response?{...emptyUnread(),...await response.json()}:emptyUnread();const kind=kindForURL(url);state[kind]++;
  await cache.put(unreadURL(),new Response(JSON.stringify(state),{headers:{'Content-Type':'application/json'}}));
  const total=state.messages+state.diagnostics+state.phone;if(self.navigator.setAppBadge)await self.navigator.setAppBadge(total);
  const windows=await clients.matchAll({type:'window',includeUncontrolled:true});for(const client of windows)client.postMessage(state);
}
function markUnread(url) { unreadUpdate=unreadUpdate.catch(()=>{}).then(()=>updateUnread(url));return unreadUpdate }

self.addEventListener('activate', event => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', event => {
  const data = event.data ? event.data.json() : {};
  event.waitUntil(Promise.all([
    self.registration.showNotification(data.title || '新信息', {
      body: data.body || '收到一条新信息',
      tag: data.tag || 'new-message',
      icon: '/icons/icon-192.png',
      badge: '/icons/badge-96.png',
      data: {url: data.url || '/?screen=messages'}
    }),
    markUnread(data.url)
  ]));
});

self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil((async () => {
    const url = new URL(event.notification.data?.url || '/?screen=messages', self.location.origin).href;
    const windows = await clients.matchAll({type:'window',includeUncontrolled:true});
    const existing = windows[0];
    if (existing) { await existing.navigate(url); return existing.focus(); }
    return clients.openWindow(url);
  })());
});

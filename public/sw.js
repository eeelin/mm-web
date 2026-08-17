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
    self.registration.setAppBadge ? self.registration.setAppBadge() : Promise.resolve()
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

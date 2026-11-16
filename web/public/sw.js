// SOMUN service worker: shell cached, data never cached (D33).
// /api/ is network-only: no operating data stays in the browser cache,
// and offline shows an error instead of stale data.
// Bump CACHE on each release that changes cached assets.
const CACHE = 'somun-v4';

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(['/'])).then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const path = new URL(event.request.url).pathname;
  if (path.startsWith('/api/')) return;
  event.respondWith(
    fetch(event.request).then((response) => {
      const copy = response.clone();
      caches.open(CACHE).then((cache) => cache.put(event.request, copy));
      return response;
    }).catch(() =>
      caches.match(event.request).then((hit) => hit || caches.match('/')),
    ),
  );
});

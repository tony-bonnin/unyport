const CACHE_NAME = "unyport-shell-v1";
const CACHEABLE_PREFIXES = [
  "/app/",
  "/css/",
  "/media/",
  "/vendor/",
  "/webfonts/"
];
const CACHEABLE_PATHS = new Set([
  "/",
  "/favicon.ico",
  "/manifest.json"
]);

self.addEventListener("install", (event) => {
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") return;

  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/sse/")) return;

  const cacheable =
    CACHEABLE_PATHS.has(url.pathname) ||
    CACHEABLE_PREFIXES.some((prefix) => url.pathname.startsWith(prefix));

  if (!cacheable) return;

  event.respondWith(
    fetch(event.request)
      .then((response) => {
        if (response.ok) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
        }
        return response;
      })
      .catch(() => caches.match(event.request))
  );
});

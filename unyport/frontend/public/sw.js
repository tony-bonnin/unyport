const CACHE_NAME = "unyport-shell-v2";
const OFFLINE_URL = "/offline.html";
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
  "/manifest.json",
  OFFLINE_URL
]);

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) => cache.addAll(["/", OFFLINE_URL, "/manifest.json", "/favicon.ico", "/media/img/icons/unyport-icon.svg"]))
      .then(() => self.skipWaiting())
  );
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

  const isNavigation = event.request.mode === "navigate";

  const cacheable =
    CACHEABLE_PATHS.has(url.pathname) ||
    CACHEABLE_PREFIXES.some((prefix) => url.pathname.startsWith(prefix));

  if (!cacheable && !isNavigation) return;

  event.respondWith(
    fetch(event.request)
      .then((response) => {
        if (response.ok && cacheable) {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
        }
        return response;
      })
      .catch(() => {
        if (isNavigation) return caches.match(OFFLINE_URL);
        return caches.match(event.request);
      })
  );
});

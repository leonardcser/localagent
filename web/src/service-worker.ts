/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

import { build, files, version } from "$service-worker";

const sw = self as unknown as ServiceWorkerGlobalScope;
const CACHE_NAME = `app-${version}`;

// Assets to precache: build output (JS/CSS) + static files (icons, manifest)
const precacheUrls = [...build, ...files];

// --- Install: precache app shell ---
sw.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) => cache.addAll(precacheUrls))
      .then(() => sw.skipWaiting()),
  );
});

// --- Activate: clean old caches, claim clients ---
sw.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter((k) => k.startsWith("app-") && k !== CACHE_NAME)
            .map((k) => caches.delete(k)),
        ),
      )
      .then(() => sw.clients.claim()),
  );
});

// --- Fetch: serve from cache, fall back to network ---
sw.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);

  // Skip non-GET, API calls, and SSE
  if (event.request.method !== "GET") return;
  if (url.pathname.startsWith("/api/")) return;

  // For navigation requests: network-first, fall back to cached shell.
  // Cache-first breaks on iOS — Safari's SW context can fail to restore after
  // backgrounding, serving a corrupt/empty response (WebKit #211018).
  if (event.request.mode === "navigate") {
    event.respondWith(
      fetch(event.request)
        .then((response) => {
          // Update cached shell with fresh copy
          const clone = response.clone();
          caches
            .open(CACHE_NAME)
            .then((cache) => cache.put("/index.html", clone));
          return response;
        })
        .catch(() => caches.match("/index.html"))
        .then((r) => r || fetch(event.request)),
    );
    return;
  }

  // For all other assets: cache-first
  event.respondWith(
    caches.match(event.request).then((cached) => {
      if (cached) return cached;
      return fetch(event.request).then((response) => {
        // Cache immutable build assets on the fly
        if (response.ok && url.pathname.startsWith("/_app/immutable/")) {
          const clone = response.clone();
          caches
            .open(CACHE_NAME)
            .then((cache) => cache.put(event.request, clone));
        }
        return response;
      });
    }),
  );
});

// --- Push notifications ---
sw.addEventListener("push", (event: PushEvent) => {
  if (!event.data) return;

  const data = event.data.json() as {
    type?: string;
    title?: string;
    body?: string;
    url?: string;
    taskId?: string;
  };
  const type = data.type || "chat";
  const title = data.title || "localagent";
  const body = data.body || "";
  const url = data.url || "/";

  if (type === "reminder") {
    event.waitUntil(
      sw.registration.showNotification(title, {
        body,
        tag: `reminder-${data.taskId ?? "unknown"}`,
        requireInteraction: true,
        data: { url },
      }),
    );
    return;
  }

  event.waitUntil(
    sw.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((windowClients) => {
        const chatActive = windowClients.some((c) => {
          const wc = c as WindowClient;
          const onChat = new URL(wc.url).pathname === "/";
          return onChat && (wc.focused || wc.visibilityState === "visible");
        });
        if (chatActive) return;
        return sw.registration.showNotification(title, {
          body,
          tag: "localagent-message",
          data: { url },
        });
      }),
  );
});

// --- Notification click ---
sw.addEventListener("notificationclick", (event: NotificationEvent) => {
  event.notification.close();
  const url = (event.notification.data?.url as string) || "/";

  event.waitUntil(
    sw.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((clients) => {
        for (const client of clients) {
          if (new URL(client.url).pathname === url && "focus" in client) {
            return (client as WindowClient).focus();
          }
        }
        return sw.clients.openWindow(url);
      }),
  );
});

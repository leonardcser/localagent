/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

const sw = self as unknown as ServiceWorkerGlobalScope;

// Activate immediately, claim all clients.
// No caching, no fetch interception — the Go server handles everything.
// This SW exists solely for push notifications.
sw.addEventListener("install", () => sw.skipWaiting());
sw.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.map((k) => caches.delete(k))))
      .then(() => sw.clients.claim()),
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

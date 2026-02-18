/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

const sw = self as unknown as ServiceWorkerGlobalScope;

sw.addEventListener("push", (event: PushEvent) => {
	if (!event.data) return;

	const data = event.data.json() as {
		title?: string;
		body?: string;
		url?: string;
	};
	const title = data.title || "localagent";
	const body = data.body || "";
	const url = data.url || "/";

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

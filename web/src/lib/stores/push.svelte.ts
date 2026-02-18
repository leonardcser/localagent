import { getVAPIDPublicKey, subscribePush } from "$lib/api";

function createPush() {
	let permission = $state<NotificationPermission>(
		typeof Notification !== "undefined" ? Notification.permission : "default",
	);

	const supported =
		typeof window !== "undefined" &&
		"serviceWorker" in navigator &&
		"PushManager" in window &&
		"Notification" in window;

	function refreshPermission() {
		if (typeof Notification !== "undefined") {
			permission = Notification.permission;
		}
	}

	async function subscribe() {
		if (!supported) return;

		refreshPermission();

		if (permission === "denied") {
			window.alert(
				"Notifications are blocked. Enable them in your browser or system notification settings, then try again.",
			);
			return;
		}

		try {
			const perm = await Notification.requestPermission();
			permission = perm;
			if (perm !== "granted") return;

			const reg = await navigator.serviceWorker.ready;
			const keyRes = await getVAPIDPublicKey();
			if (!keyRes) return;

			const sub = await reg.pushManager.subscribe({
				userVisibleOnly: true,
				applicationServerKey: urlBase64ToUint8Array(keyRes)
					.buffer as ArrayBuffer,
			});

			await subscribePush(sub);
		} catch {
			// subscription failed
		}
	}

	return {
		get permission() {
			return permission;
		},
		get supported() {
			return supported;
		},
		subscribe,
	};
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
	const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
	const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
	const raw = atob(base64);
	const arr = new Uint8Array(raw.length);
	for (let i = 0; i < raw.length; i++) {
		arr[i] = raw.charCodeAt(i);
	}
	return arr;
}

export const push = createPush();

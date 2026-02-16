import adapter from "@sveltejs/adapter-static";

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		adapter: adapter({
			pages: "../pkg/webchat/static",
			assets: "../pkg/webchat/static",
			fallback: "index.html",
			precompress: false,
		}),
	},
};

export default config;

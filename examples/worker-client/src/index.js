export default {
	async fetch(request, env) {
		const url = new URL(request.url);
		if (url.pathname === "/health") {
			return new Response("worker ok");
		}
		if (url.pathname === "/live") {
			const origin = await env.COGS_ORIGIN.fetch("http://127.0.0.1:8080/api/hello", {
				headers: { "X-Cogs-Private-Token": env.COGS_PRIVATE_TOKEN || "" },
			});
			return origin;
		}
		if (url.pathname === "/edge/items") {
			const { results } = await env.DB.prepare("SELECT id, val FROM items_edge LIMIT 20").all();
			return Response.json({ results });
		}
		return new Response("not found", { status: 404 });
	},
};

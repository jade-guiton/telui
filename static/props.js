const propTemplate = document.querySelector("#prop-template");

function renderProp(ctx, k, v) {
	const propContent = propTemplate.content.cloneNode(true);
	if(k != null) {
		propContent.querySelector(".prop-key").innerText = k;
	}
	const typeNode = propContent.querySelector(".prop-type");
	const inlineValueNode = propContent.querySelector(".prop-inline-value");
	const childrenNode = propContent.querySelector(".prop-children");

	let type = "?";
	let inline = "?";
	let children = [];
	if(typeof v == "string") {
		type = "str";
		if(v.length <= 60) {
			inline = JSON.stringify(v);
		} else {
			inline = "";
			const preNode = document.createElement("pre");
			preNode.innerText = v;
			children = [preNode];
		}
	} else if(typeof v == "bigint") {
		type = "int"; inline = ""+v;
	} else if(typeof v == "number") {
		type = "float"; inline = ""+v;
	} else if(typeof v == "boolean") {
		type = "bool"; inline = ""+v;
	} else if(v instanceof Array) {
		type = "array"; inline = "";
		children = v.map(item => renderProp(ctx, null, item));
	} else if(typeof v == "object" && v != null) {
		if(v._ts) {
			type = "time"; inline = timestamp(v._ts);
		} else if(v._byt) {
			type = "bytes"; inline = v._byt;
		} else if(v._res) {
			type = "res";
			if(ctx.resource) {
				inline = "";
				children = renderMap(ctx, ctx.resource);
			} else if(ctx.resources && ctx.resources[v._res]) {
				inline = "";
				children = renderMap(ctx, ctx.resources[v._res]);
			} else {
				inline = v._res;
			}
		} else if(v._scope) {
			type = "scope";
			if(ctx.scope) {
				inline = "";
				children = renderMap(ctx, ctx.scope);
			} else if(ctx.scopes && ctx.scopes[v._scope]) {
				inline = "";
				children = renderMap(ctx, ctx.scopes[v._scope]);
			} else {
				inline = v._scope;
			}
		} else if(v._span) {
			type = "span";
			const trace = v._trace ?? ctx.traceId;
			const span = v._span;
			if(trace) {
				inline = trace + " / " + span;
				const link = document.createElement("a");
				link.innerText = "[go to span]";
				link.addEventListener("click", ev => {
					selectSpan(trace, span);
					ev.preventDefault();
				});
				children = [link];
			} else {
				inline = span;
			}
		} else {
			type = "map"; inline = "";
			children = renderMap(ctx, v);
		}
	}
	typeNode.innerText = type;
	inlineValueNode.innerText = inline;
	childrenNode.replaceChildren(...children);
	return propContent;
}

function renderMap(ctx, map) {
	const pairs = Object.entries(map);
	return pairs.map(([k,v]) => {
		if(k.startsWith("__")) {
			k = k.slice(1)
		} else if(k.startsWith("_")) {
			console.warn(`Unhandled reserved key: ${k}`)
		}
		return renderProp(ctx, k, v);
	});
}

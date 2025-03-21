async function updateTraces() {
	const traces = Object.entries(await fetchData("/api/traces")).map(([tid, trace]) => {
		return {
			id: tid,
			start: bigMin(...Object.values(trace).map(s => s.start._ts)),
			end: bigMax(...Object.values(trace).map(s => s.end._ts)),
			spans: trace,
		};
	});
	traces.sort((t1, t2) => cmp(t1.start, t2.start) || cmp(t1.id, t2.id));
	const traceTemplate = document.querySelector("#trace-template");
	const spanTemplate = document.querySelector("#span-template");
	document.querySelector(`#body`).replaceChildren(
		...(traces.length == 0 ? [document.createTextNode("No traces.")] : traces.map(trace => {
			for(const [sid, span] of Object.entries(trace.spans)) {
				span.id = sid;
			}
			const spanList = Object.values(trace.spans);
			spanList.sort((s1, s2) => cmp(s1.start._ts, s2.start._ts) || cmp(s1.id, s2.id));
			
			const traceNode = traceTemplate.content.cloneNode(true);
			traceNode.querySelector(".trace-id").innerText = trace.id;
			traceNode.querySelector(".trace-start").innerText = timestamp(trace.start);
			traceNode.querySelector(".trace-end").innerText = timestamp(trace.end);
			let traceDur = Number(trace.end - trace.start);
			if(traceDur <= 0) traceDur = 1;
			traceNode.querySelector(".spans").replaceChildren(...spanList.map(span => {
				const spanContent = spanTemplate.content.cloneNode(true);
				
				spanContent.querySelector(".span-id").innerText = span.id;
				spanContent.querySelector(".span-name").innerText = span.name;

				const spanBar = spanContent.querySelector(".span-bar");
				const startPercent = Number(span.start._ts - trace.start) / traceDur * 100;
				const durationPercent = Number(span.end._ts - span.start._ts) / traceDur * 100;
				spanBar.style.left = startPercent + "%";
				spanBar.style.width = durationPercent + "%";
				if(span.status == "Error") spanBar.classList.add("span-error");

				const spanNode = spanContent.querySelector(".span");
				spanNode.id = `item-span-${trace.id}-${span.id}`;

				spanNode.addEventListener("click", () => {
					selectSpan(trace.id, span.id);
				});

				return spanContent;
			}))
			return traceNode;
		}))
	);
	updateSelectedItems();
}

async function selectSpan(traceId, spanId) {
	selectItem(`span-${traceId}-${spanId}`, `Trace ${traceId} | Span ${spanId}`);
	
	let data;
	try {
		data = await fetchData(`/api/span/${traceId}/${spanId}`);
	} catch(err) {
		setPanelBody([document.createTextNode(
			"Failed to load span" + (err.statusCode == 404 ? ": span does not exist" : "")
		)]);
		console.error(err);
		return;
	}
	data.traceId = traceId;
	setPanelBody(renderMap(data, data.span));
}

async function updateLogs() {
	const logs = await fetchData("/api/logs");
	for(let i = 0; i < logs.length; i++) {
		logs[i].id = i;
	}
	logs.sort((l1, l2) => cmp(l1.time._ts, l2.time._ts));
	const logTemplate = document.querySelector("#log-template");
	document.querySelector(`#body`).replaceChildren(
		...(logs.length == 0 ? [document.createTextNode("No logs.")] : logs.map(log => {
			const logContent = logTemplate.content.cloneNode(true);
			const logNode = logContent.querySelector(".log");
			logContent.querySelector(".log-time").innerText = timestamp(log.time._ts, true);
			logContent.querySelector(".log-sev").innerText = log.sev;
			logContent.querySelector(".log-body").innerText = log.body;
			if(log.sev.startsWith("Debug")) logNode.classList.add("log-debug");
			if(log.sev.startsWith("Warn")) logNode.classList.add("log-warn");
			if(log.sev.startsWith("Error")) logNode.classList.add("log-error");
			logNode.id = `item-log-${log.id}`;
			logNode.addEventListener("click", () => {
				console.log("Test");
				selectLog(log.id);
			});
			return logContent;
		}))
	);
	updateSelectedItems();
}

async function selectLog(logId) {
	selectItem(`log-${logId}`, `Log ${logId}`);
	
	let data;
	try {
		data = await fetchData(`/api/log/${logId}`);
	} catch(err) {
		setPanelBody([document.createTextNode("Failed to load log")]);
		console.error(err);
		return;
	}

	setPanelBody(renderMap(data, data.log));
}

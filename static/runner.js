const liveCheckbox = document.querySelector("#live");
let loopPromise;
let stopLoop;
async function updateLoop(updater) {
	let firstUpdate = true;
	let running = true;
	while(running) {
		stopLoop = () => running = false;
		if(liveCheckbox.checked || firstUpdate) {
			try {
				await updater();
				await updatePanel();
			} catch(err) {
				console.error("Failed to update UI:", err);
			}
			firstUpdate = false;
		}
		if(!running) break;
		running = await new Promise(resolve => {
			const timeoutId = setTimeout(() => resolve(true), 500);
			stopLoop = () => { clearTimeout(timeoutId); resolve(false); };
		});
	}
}
function startUpdating(updater) {
	loopPromise = updateLoop(updater);
}
async function stopUpdating() {
	if(loopPromise) {
		stopLoop();
		await loopPromise;
		loopPromise = undefined;
	}
};

const tabs = {
	"#traces": {
		tabId: "traces-tab",
		title: "Traces - TelUI",
		updater: updateTraces,
	},
	"#logs": {
		tabId: "logs-tab",
		title: "Logs - TelUI",
		updater: updateLogs,
	},
	"#metrics": {
		tabId: "metrics-tab",
		title: "Metrics - TelUI",
		updater: updateMetrics,
	},
}
const body = document.querySelector(`#body`);
let updatingTab = false;
async function updateTab() {
	if(updatingTab) return;
	updatingTab = true;

	await stopUpdating();

	if(!tabs[location.hash]) {
		location.hash = "#traces";
	}
	const tab = tabs[location.hash];
	for(const tabNode of document.querySelectorAll(".tab")) {
		tabNode.classList.remove("active-tab");
	}
	document.querySelector(`#${tab.tabId}`).classList.add("active-tab");
	body.innerText = "Loading...";
	document.title = tab.title;

	startUpdating(tab.updater);
	updatingTab = false;
}
addEventListener("load", updateTab);
addEventListener("hashchange", updateTab);

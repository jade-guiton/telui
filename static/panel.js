const panel = document.querySelector("#panel");
const panelTitle = document.querySelector("#panel-title");
const panelBody = document.querySelector("#panel-body");

let selectedItemId = undefined;
let panelUpdater = undefined;

function updateSelectedItems() {
	for(const el of document.querySelectorAll(".selected")) {
		el.classList.remove("selected");
	}
	if(selectedItemId) {
		const itemNode = document.querySelector("#item-" + selectedItemId);
		if(itemNode) {
			itemNode.classList.add("selected");
		}
	}
}

function selectItem(itemId, title) {
	selectedItemId = itemId;
	panelUpdater = undefined;
	updateSelectedItems();

	panelTitle.innerText = title;
	panelBody.innerText = "Loading...";
	panel.hidden = false;
}

function setPanelBody(children) {
	panelBody.replaceChildren(...children);
}

function setPanelUpdater(updater) {
	panelUpdater = updater;
}

async function updatePanel() {
	if(panelUpdater) {
		await panelUpdater();
	}
}

document.querySelector("#panel-close").addEventListener("click", () => {
	selectedItemId = undefined;
	panelUpdater = undefined;
	updateSelectedItems();
	panel.hidden = true;
})

const resizer = document.querySelector("#panel-resizer");
resizer.addEventListener("mousedown", ev => {
	function mousemove(ev) {
		panel.style.height = (window.innerHeight - ev.y) + "px";
	}
	function mouseup() {
		window.removeEventListener("mousemove", mousemove);
		window.removeEventListener("mouseup", mouseup);
	}
	window.addEventListener("mousemove", mousemove);
	window.addEventListener("mouseup", mouseup);
});
resizer.addEventListener("mouseup", () => { dragging = false });

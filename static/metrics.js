function sortedEntries(obj) {
	const arr = Object.entries(obj);
	arr.sort(([k1,], [k2,]) => cmp(k1, k2));
	return arr;
}

async function updateMetrics() {
	const data = await fetchData("/api/metrics");

	const resMap = {};
	for(const [mid, metric] of Object.entries(data.metrics)) {
		const resId = metric.res._res;
		const scopeId = metric.scope._scope;
		delete metric.res;
		delete metric.scope;
		if(!resMap[resId]) resMap[resId] = {};
		const res = resMap[resId];
		if(!res[scopeId]) res[scopeId] = {};
		const scope = res[scopeId];
		scope[mid] = metric;
	}
	const resArr = sortedEntries(resMap);

	const resourceTemplate = document.querySelector("#resource-template");
	const scopeTemplate = document.querySelector("#scope-template");
	const metricTemplate = document.querySelector("#metric-template");
	document.querySelector(`#body`).replaceChildren(
		...(resArr.length == 0 ? [document.createTextNode("No metrics.")] : resArr.map(([resId, scopes]) => {
			const resourceContent = resourceTemplate.content.cloneNode(true);
			resourceContent.querySelector(".resource-props").replaceChildren(...renderMap(data, data.resources[resId]));
			resourceContent.querySelector(".resource-items").replaceChildren(...sortedEntries(scopes).map(([scopeId, metrics]) => {
				const scopeContent = scopeTemplate.content.cloneNode(true);
				scopeContent.querySelector(".scope-props").replaceChildren(...renderMap(data, data.scopes[scopeId]));
				scopeContent.querySelector(".scope-items").replaceChildren(...sortedEntries(metrics).map(([metricId, metric]) => {
					const metricContent = metricTemplate.content.cloneNode(true);

					metricContent.querySelector(".metric-name").innerText = metric.name;

					let unit = metric.unit ?? "";
					if(unit.startsWith("{") && unit.endsWith("}"))
						unit = unit.slice(1, -1);
					metricContent.querySelector(".metric-unit").innerText = unit;

					let type = {
						"Gauge": "gauge",
						"Sum": "sum",
						"Histogram": "histo",
						"ExponentialHistogram": "exp-histo",
						"Summary": "summary",
					}[metric.type];
					if(["Sum", "Histogram", "ExponentialHistogram"].includes(metric.type)) {
						type += " " + {"Delta": "Δ", "Cumulative": "Σ"}[metric.tempo];
					}
					if(metric.type == "Sum" && metric.mono) {
						type += " ↗";
					}
					metricContent.querySelector(".metric-type").innerText = type;

					metricContent.querySelector(".metric-desc").innerText = metric.desc;

					const metricNode = metricContent.querySelector(".metric");
					metricNode.id = `item-metric-${metricId}`;
					metricNode.addEventListener("click", () => {
						selectMetric(metricId, metric);
					});
					return metricContent;
				}));
				return scopeContent;
			}));
			return resourceContent;
		}))
	);
	updateSelectedItems();
}

async function selectMetric(metricId, metric) {
	selectItem(`metric-${metricId}`, `Metric ${metric.name} (${metricId})`);
	
	const panelUpdater = async () => {
		let data;
		try {
			data = await fetchData(`/api/metric/${metricId}`);
		} catch(err) {
			setPanelBody([document.createTextNode("Failed to load metric")]);
			console.error(err);
			return;
		}

		const metric = data.metric;

		let children = [];
		if(metric.conflict) {
			const conflictNode = document.createElement("span");
			conflictNode.classList.add("metric-conflict");
			conflictNode.innerText = "Conflicting description or metadata was received";
			children.push(conflictNode);
		}
		if(metric.meta) {
			children.push(renderProp(data, "meta", metric.meta));
		}
		
		const streamTemplate = document.querySelector("#metric-stream-template");
		children.push(...sortedEntries(metric.streams).map(([streamId, stream]) => {
			const streamNode = streamTemplate.content.cloneNode(true);
			streamNode.querySelector(".metric-stream-attrs").replaceChildren(...renderMap(data, stream.attr));
			const graph = Graph.getGraph(streamId, streamNode.querySelector(".metric-stream-points"));
			graph.setContext(data);
			
			if(metric.type == "Gauge" || metric.type == "Sum") {
				for(const pt of stream.pts) {
					graph.addPoint("#0f0", pt.time._ts, pt.val, pt);
				}
			} else if(metric.type == "Histogram" || metric.type == "ExponentialHistogram" || metric.type == "Summary") {

				for(const pt of stream.pts) {

					if(metric.type == "Histogram" && pt.buckets) {
						const newBuckets = [];
						for(let i = 0; i < pt.buckets.length; i++) {
							if(pt.buckets[i] == 0) continue;
							const bucket = {
								cnt: pt.buckets[i],
							};
							if(i > 0) {
								bucket.min = pt.bounds[i-1];
							}
							if(i < pt.bounds.length) {
								bucket.max = pt.bounds[i];
							}
							newBuckets.push(bucket);
						}
						pt.buckets = newBuckets;
						delete pt.bounds;

					} else if(metric.type == "ExponentialHistogram") {
						const powScale = Math.pow(2, -Number(pt.scale));
						const lowerBound = index => Math.pow(2, index * powScale);
						const newBuckets = [];

						const negOff = Number(pt["neg.off"]);
						for(let i = pt.neg.length-1; i >= 0; i--) {
							if(pt.neg[i] == 0) continue;
							newBuckets.push({
								min: -lowerBound(negOff + i + 1),
								max: -lowerBound(negOff + i),
								cnt: pt.neg[i],
							});
						}

						if(pt.zeros != 0) {
							const thre = pt["zeros.thre"] ?? 0;
							newBuckets.push({
								min: thre,
								max: thre,
								cnt: pt.zeros,
							})
						}

						const posOff = Number(pt["pos.off"]);
						for(let i = 0; i < pt.pos.length; i++) {
							if(pt.pos[i] == 0) continue;
							newBuckets.push({
								min: lowerBound(posOff + i),
								max: lowerBound(posOff + i + 1),
								cnt: pt.pos[i],
							});
						}

						delete pt["neg.off"];
						delete pt.neg;
						delete pt.zeros;
						delete pt["zeros.thre"];
						delete pt["pos.off"];
						delete pt.pos;
						pt.buckets = newBuckets;

					} else if(metric.type == "Summary") {
						for(const qv of pt.quantiles) {
							if(qv.q == 0) pt.min = qv.v;
							if(qv.q == 1) pt.max = qv.v;
						}
					}

					graph.addPoint("#888", pt.time._ts, undefined, pt);
					if(pt.min != undefined) {
						graph.addPoint("#f0f", pt.time._ts, pt.min);
					}
					if(pt.max != undefined) {
						graph.addPoint("#0ff", pt.time._ts, pt.max);
					}
					if(pt.sum != undefined) {
						graph.addPoint("#ff0", pt.time._ts, pt.sum / Number(pt.cnt));
					}
				}
			}

			graph.render();

			return streamNode;
		}));

		setPanelBody(children);
	};

	setPanelUpdater(panelUpdater);
	panelUpdater();
}

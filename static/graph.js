function formatValue(x) {
	if(typeof x == "number") {
		return x.toPrecision(6);
	} else {
		return ""+x;
	}
}

const graphCache = new Map();
const graphTemplate = document.querySelector("#graph-template");

class Graph {
	static getGraph(graphId, container) {
		let graph = graphCache.get(graphId);
		if(graph != undefined) {
			graph = graph.deref();
		}
		if(graph == undefined) {
			graph = new Graph();
			graphCache.set(graphId, new WeakRef(graph));
		}
		graph.reset(container);
		return graph;
	}

	constructor() {
		this.mousePos = null;

		const graphContent = graphTemplate.content.cloneNode(true)
		this.graphNode = graphContent.querySelector(".graph");
		this.canvas = this.graphNode.querySelector(".graph-canvas");
		this.labels = {
			minTime: this.graphNode.querySelector(".graph-min-time"),
			maxTime: this.graphNode.querySelector(".graph-max-time"),
			minValue: this.graphNode.querySelector(".graph-min-value"),
			maxValue: this.graphNode.querySelector(".graph-max-value"),
		};
		this.pointProps = this.graphNode.querySelector(".graph-point-props");

		new ResizeObserver(() => {
			if(this.canvas.clientWidth == 0 || this.canvas.clientHeight == 0) return;
			this.canvas.width = this.canvas.clientWidth;
			this.canvas.height = this.canvas.clientHeight;
			this.render();
		}).observe(this.canvas);
		const mouseIn = ev => {
			if(this.defocusTimeout != undefined) {
				clearTimeout(this.defocusTimeout);
				delete this.defocusTimeout;
			}
			this.mousePos = { x: ev.offsetX, y: ev.offsetY };
			this.render();
		};
		this.canvas.addEventListener("mousemove", mouseIn);
		this.canvas.addEventListener("mouseenter", mouseIn);
		this.graphNode.addEventListener("mouseleave", () => {
			this.defocusTimeout = setTimeout(() => {
				this.mousePos = null;
				this.render();
			}, 250);
		});
	}

	reset(container) {
		this.minTime = null;
		this.maxTime = null;
		this.minValue = null;
		this.maxValue = null;
		this.points = [];
		this.ctx = null;
		container.appendChild(this.graphNode);
	}

	setContext(ctx) {
		this.ctx = ctx;
	}

	addPoint(style, time, value, props) {
		if(this.minTime == null || time < this.minTime)
			this.minTime = time;
		if(this.maxTime == null || time > this.maxTime)
			this.maxTime = time;
		if(value != undefined) {
			if(this.minValue == null || value < this.minValue)
				this.minValue = value;
			if(this.maxValue == null || value > this.maxValue)
				this.maxValue = value;
		}
		this.points.push({time, value, style, props});
	}

	render() {
		const w = this.canvas.width;
		const h = this.canvas.height

		const ctx = this.canvas.getContext("2d");
		ctx.fillStyle = "#222";
		ctx.fillRect(0, 0, w, h);

		const ptSz = 5;
		const lineSz = 1;
		const gap = 10;
		const wIn = w - 2*gap;
		const hIn = h - 2*gap;

		let timeInterval;
		if(this.maxTime == this.minTime) {
			this.minTime--;
			this.maxTime++;
			timeInterval = 2;
		} else {
			timeInterval = Number(this.maxTime - this.minTime);
		}
		let valueInterval;
		if(this.minValue != null) {
			if(this.maxValue == this.minValue) {
				this.minValue -= typeof this.minValue == "bigint" ? 1n : 0.5;
				this.maxValue += typeof this.maxValue == "bigint" ? 1n : 0.5;
				valueInterval = 2;
			} else {
				valueInterval = Number(this.maxValue - this.minValue);
			}
		}

		const pointX = pt =>
			gap + Number(pt.time - this.minTime) / timeInterval * wIn;
		const pointY = pt =>
			gap + Number(this.maxValue - pt.value) / valueInterval * hIn;
		
		let bestDist = Infinity;
		let focus;
		if(this.mousePos) {
			const mx = this.mousePos.x; const my = this.mousePos.y;
			for(const pt of this.points) {
				if(pt.props != undefined) {
					const x = pointX(pt);
					let dist = (x-mx)*(x-mx);
					if(pt.value != undefined) {
						const y = pointY(pt);
						dist += (y-my)*(y-my);
					}
					if(dist <= bestDist) {
						bestDist = dist;
						focus = pt;
					}
				}
			}
		}

		const drawDiamond = (x, y, sz, style) => {
			ctx.fillStyle = style;
			ctx.beginPath();
			ctx.moveTo(x-sz, y);
			ctx.lineTo(x, y-sz);
			ctx.lineTo(x+sz, y);
			ctx.lineTo(x, y+sz);
			ctx.fill();
		};
		const drawLine = (x, sz, style) => {
			ctx.lineWidth = sz;
			ctx.strokeStyle = style;
			ctx.beginPath();
			ctx.moveTo(x, 0);
			ctx.lineTo(x, h);
			ctx.stroke();
		};

		for(const pt of this.points) {
			const x = pointX(pt);
			if(pt.value != undefined) {
				const y = pointY(pt);
				if(pt == focus) {
					drawDiamond(x, y, ptSz * 1.5, "#fff");
				}
				drawDiamond(x, y, ptSz, pt.style);
			} else {
				if(pt == focus) {
					drawLine(x, lineSz * 2, "#fff");
				}
				drawLine(x, lineSz, pt.style);
			}
		}
		
		this.labels.minTime.innerText = timestamp(this.minTime);
		this.labels.maxTime.innerText = timestamp(this.maxTime);
		this.labels.minValue.innerText = formatValue(this.minValue);
		this.labels.maxValue.innerText = formatValue(this.maxValue);

		if(focus) {
			this.pointProps.replaceChildren(...renderMap(this.ctx, focus.props));
		} else {
			this.pointProps.replaceChildren();
		}
	}
}
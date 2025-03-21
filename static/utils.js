function bigMin(...xs) {
	let min;
	for(const x of xs) {
		if(min == undefined || x < min) min = x;
	}
	return min;
}
function bigMax(...xs) {
	let max;
	for(const x of xs) {
		if(max == undefined || x > max) max = x;
	}
	return max;
}
function cmp(n1, n2) {
	return n1 > n2 ? 1 : n1 < n2 ? -1 : 0;
}
function timestamp(t, short) {
	const ms = new Date(Number(t/1000000n)).toISOString().replace("T", " ").slice(0, -1);
	if(short) return ms;
	const ns = String(t%1000000n).padStart(6,"0");
	return ms + " " + ns.slice(0,3) + " " + ns.slice(3) + " UTC"
}

async function fetchData(url) {
	const res = await fetch(url);
	if(!res.ok) {
		const err = new Error(`Request returned code ${res.status}`);
		err.statusCode = res.status;
		throw err;
	}
	const data = await res.text();
	return JSON.parse(data, function(k, v) {
		if(v._int != undefined) {
			return BigInt(v._int);
		} else if(v._ts != undefined) {
			return {_ts: BigInt(v._ts)}
		} else {
			return v
		}
	});
}

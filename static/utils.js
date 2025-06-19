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
	if(n1 == undefined) n1 = [];
	if(n2 == undefined) n2 = [];
	if(!(n1 instanceof Array)) n1 = [n1];
	if(!(n2 instanceof Array)) n2 = [n2];
	for(let i = 0; i < Math.max(n1.length, n2.length); i++) {
		const x1 = n1[i];
		const x2 = n2[i];
		const t1 = typeof x1;
		const t2 = typeof x1;
		if(t1 != t2) return t1 > t2 ? 1 : -1;
		if(x1 != x2) return x1 > x2 ? 1 : -1;
	}
	return 0;
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

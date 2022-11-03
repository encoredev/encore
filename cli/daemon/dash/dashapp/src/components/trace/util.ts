const unselected = [
  "#93C0FF", // Encore Light Blue
  "#FFB84A", // Code Orange
  "#B3D77E", // Code Green
  "#6E89FF", // Code Blue
  "#A36C8C", // Code Purple
];
const selected = [
  "#AACCF8", // Encore Light Blue Highlight
  "#FBC674", // Code Orange Highlight
  "#C2DD98", // Code Green Highlight
  "#8EA3F8", // Code Blue Highlight
  "#B68DA2", // Code Purple Highlight
];

export function svcColor(svc: string): [string, string] {
  const n = unselected.length;
  let idx = strhash(svc) % n;
  if (idx < 0) idx += n;
  return [unselected[idx], selected[idx]];
}

export function idxColor(idx: number): [string, string] {
  const n = unselected.length;
  idx = idx % n;
  if (idx < 0) idx += n;
  return [unselected[idx], selected[idx]];
}

function strhash(s: string): number {
  let hash = 0;
  for (var i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    hash = (hash << 5) - hash + c;
    hash &= hash; // Convert to 32bit integer
  }
  return hash;
}

export function latencyStr(n: number): string {
  if (n < 1000) {
    return Math.round(n) + "µs";
  }
  n /= 1000;

  if (n < 1000) {
    return Math.round(n) + "ms";
  }
  n /= 1000;

  if (n < 10) {
    return Math.round(n * 10) / 10 + "s";
  } else if (n < 3600) {
    return Math.round(n) + "s";
  }

  n /= 3600;
  if (n < 10) {
    return Math.round(n * 10) / 10 + "h";
  }
  return Math.round(n) + "h";
}

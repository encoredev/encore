const unselected = ["#fde8e8", "#feecdc", "#fdf6b2", "#def7ec", "#d5f5f6", "#e1effe", "#e5edff", "#edebfe", "#fce8f3"]
const selected =   ["#fbd5d5", "#fcd9bd", "#fce96a", "#bcf0da", "#afecef", "#c3ddfd", "#cddbfe", "#dcd7fe", "#fad1e8"]

export function svcColor(svc: string): [string, string] {
  const n = unselected.length
  let idx = strhash(svc) % n
  if (idx < 0) idx += n
  return [unselected[idx], selected[idx]]
}

function strhash(s: string): number {
  let hash = 0;
  for (var i = 0; i < s.length; i++) {
      const c = s.charCodeAt(i)
      hash = ((hash<<5)-hash)+c
      hash &= hash // Convert to 32bit integer
  }
  return hash
}

export function latencyStr(n: number): string {
  if (n < 1000) {
    return Math.round(n) + "µs"
  }
  n /= 1000

  if (n < 1000) {
    return Math.round(n) + "ms"
  }
  n /= 1000

  if (n < 10) {
    return (Math.round(n*10)/10) + "s"
  } else if (n < 3600) {
    return Math.round(n) + "s"
  }

  n /= 3600
  if (n < 10) {
    return (Math.round(n*10)/10) + "h"
  }
  return Math.round(n) + "h"
}
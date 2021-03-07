import {DateTime, Duration} from 'luxon';

export function timeToDate(timeStr: string): DateTime | null {
    let d = DateTime.fromISO(timeStr);
    if (d.year === 1) {
        return null;
    }
    return d;
}

export function durationStr(dur: Duration, format?: "long" | "short"): string {
    const short = format === "short"
    dur = dur.shiftTo("hours", "minutes", "seconds", "milliseconds")
    let parts: [number, string][]
    if (short) {
        parts = [[dur.hours, "h"], [dur.minutes, "m"], [dur.seconds, "s"]]
    } else {
        parts = [[dur.hours, "hour"], [dur.minutes, "minute"], [dur.seconds, "second"]]
    }

    for (var part of parts) {
        if (part[0] > 0) {
            return short ? (part[0] + part[1]) : (part[0] + " " + part[1] + (part[0] > 1 ? "s" : ""))
        }
    }
    return short ? "<1s" : "less than a second"
}
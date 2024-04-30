function roundUpTime(date, duration) {
  if (duration <= 0) {
    return date;
  }
  const rounded = new Date(Math.ceil(date.getTime() / duration) * duration);
  return rounded;
}
function roundUpBar(bar, timeframe) {
  const duration = timeframeToDuration(timeframe);
  bar.Date = roundUpTime(new Date(bar.Date), duration).getTime();
}
function isSameGroupOfSeconds(t1, t2, groupSize) {
  return Math.ceil(t1 / 1000 / groupSize) === Math.ceil(t2 / 1000 / groupSize);
}
function isSameGroupOfMinutes(t1, t2, groupSize) {
  return Math.ceil(t1 / 1000 / 60 / groupSize) === Math.ceil(t2 / 1000 / 60 / groupSize);
}

function isSameGroupOfHours(t1, t2, groupSize) {
  return Math.ceil(t1 / 1000 / 60 / 60 / groupSize) === Math.ceil(t2 / 1000 / 60 / 60 / groupSize);
}

function isSameDay(t1, t2) {
  const date1 = new Date(t1);
  const date2 = new Date(t2);
  return date1.getFullYear() === date2.getFullYear() && date1.getMonth() === date2.getMonth() && date1.getDate() === date2.getDate();
}
function isSameWeek(t1, t2) {
  const startOfWeek = d => {
    const diff = d.getDate() - d.getDay() + (d.getDay() === 0 ? -6 : 1); // adjust when day is sunday
    return new Date(d.setDate(diff));
  };
  const date1 = new Date(t1);
  const date2 = new Date(t2);
  const startOfWeek1 = startOfWeek(date1);
  const startOfWeek2 = startOfWeek(date2);
  return startOfWeek1.toISOString().slice(0, 10) === startOfWeek2.toISOString().slice(0, 10);
}
function isSameMonth(t1, t2) {
  const date1 = new Date(t1);
  const date2 = new Date(t2);
  return date1.getFullYear() === date2.getFullYear() && date1.getMonth() === date2.getMonth();
}
export function timeframeToDuration(timeframe) {
  switch (timeframe.unit) {
    case 's':
      return 1000 * timeframe.value;
    case 'm':
      return 1000 * 60 * timeframe.value;
    case 'h':
      return 1000 * 60 * 60 * timeframe.value;
    default:
      throw new Error(`timeframe unit not supported in timeframeToDuration: ${timeframe.unit}`);
  }
}

function combineBars(bar1, bar2) {
  return {
    Date: bar1.Date,
    Open: bar1.Open,
    High: Math.max(bar1.High, bar2.High),
    Low: Math.min(bar1.Low, bar2.Low),
    Close: bar2.Close,
    Volume: bar1.Volume + bar2.Volume,
  };
}

export function splitTimeframe(timeframe) {
  const match = timeframe.match(/(\d+)([a-zA-Z]+)/);
  return {
    value: parseInt(match[1], 10),
    unit: match[2],
  };
}

export function appendBar(meta, bars, bar) {
  if (bars.length === 0) {
    bars.push(bar);
  } else {
    const timeframe = splitTimeframe(meta.timeframe);
    if (timeframe.unit === "s") {
      if (isSameGroupOfSeconds(bars[bars.length - 1].Date, bar.Date, timeframe.value)) {
        bars[bars.length - 1] = combineBars(bars[bars.length - 1], bar);
      } else {
        roundUpBar(bar, timeframe);
        bars.push(bar);
      }
    } else if (timeframe.unit === "m") {
      if (isSameGroupOfMinutes(bars[bars.length - 1].Date, bar.Date, timeframe.value)) {
        bars[bars.length - 1] = combineBars(bars[bars.length - 1], bar);
      } else {
        roundUpBar(bar, timeframe);
        bars.push(bar);
      }
    } else if (timeframe.unit === "h") {
      if (isSameGroupOfHours(bars[bars.length - 1].Date, bar.Date, timeframe.value)) {
        bars[bars.length - 1] = combineBars(bars[bars.length - 1], bar);
      } else {
        roundUpBar(bar, timeframe);
        bars.push(bar);
      }
    } else if (timeframe.unit === "d") {
      if (isSameDay(bars[bars.length - 1].Date, bar.Date)) {
        bars[bars.length - 1] = combineBars(bars[bars.length - 1], bar);
      } else {
        bars.push(bar);
      }
    } else if (timeframe.unit === "w") {
      if (isSameWeek(bars[bars.length - 1].Date, bar.Date)) {
        bars[bars.length - 1] = combineBars(bars[bars.length - 1], bar);
      } else {
        bars.push(bar);
      }
    } else if (timeframe.unit === "mo") {
      if (isSameMonth(bars[bars.length - 1].Date, bar.Date)) {
        bars[bars.length - 1] = combineBars(bars[bars.length - 1], bar);
      } else {
        bars.push(bar);
      }
    } else {
      throw new Error(`timeframe unit not recognized: ${timeUnit}`);
    }
  }
}

export function roundUpToTimeframe(dt, tf) {
  const duration = timeframeToDuration(tf);
  return Math.ceil(dt / duration) * duration;
}
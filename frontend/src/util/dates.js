export function nextDay(dateMillis) {
  const date = new Date(dateMillis);
  date.setDate(date.getDate() + 1);
  return date.getTime();
}

export function prevDay(dateMillis) {
  const date = new Date(dateMillis);
  date.setDate(date.getDate() - 1);
  return date.getTime();
}

export function nextSunday(dateMillis) {
  const date = new Date(dateMillis);
  const day = date.getDay();
  const difference = 7 - day;
  date.setDate(date.getDate() + difference);
  return date.getTime();
}

export function nextMonth(dateMillis) {
  const date = new Date(dateMillis);
  const month = date.getMonth();
  date.setMonth(month + 1);
  date.setDate(1);
  return date.getTime();
}

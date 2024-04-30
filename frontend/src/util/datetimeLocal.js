export function fromDatetimeLocal(datetimeString) {
  let [date, time] = datetimeString.split('T');

  if (!date || !time) {
    return null;
  }

  let [year, month, day] = date.split('-').map(Number);
  let [hours, minutes, seconds] = time.split(':').map(Number);

  // Month in JavaScript Date() is 0-indexed, so subtract 1
  if (seconds) {
    return new Date(year, month - 1, day, hours, minutes, seconds);
  } else {
    return new Date(year, month - 1, day, hours, minutes);
  }
}

export function toDatetimeLocal(date, pretty) {
  const yyyy = date.getFullYear().toString();
  const MM = (date.getMonth() + 1).toString().padStart(2, '0');
  const dd = date.getDate().toString().padStart(2, '0');
  const HH = date.getHours().toString().padStart(2, '0');
  const mm = date.getMinutes().toString().padStart(2, '0');
  const ss = date.getSeconds().toString().padStart(2, '0');

  const sep = pretty ? ' ' : 'T';

  return `${yyyy}-${MM}-${dd}${sep}${HH}:${mm}:${ss}`;
}
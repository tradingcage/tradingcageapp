import { writable, get } from "svelte/store";
import { humanReadableSymbol, symbolsIndex } from "../util/constants.js";

const fetchData = async () => {
  try {
    const response = await fetch("/bar-ranges");
    if (!response.ok) throw new Error("Network response was not ok.");
    const data = await response.json();
    return data
      .sort((a, b) => {
        const nameA = humanReadableSymbol[symbolsIndex[a.SymbolID]];
        const nameB = humanReadableSymbol[symbolsIndex[b.SymbolID]];
        return nameA.localeCompare(nameB);
      })
      .map(({ SymbolID, FirstDate, LastDate }) => ({
        symbol_id: SymbolID,
        first_date: new Date(FirstDate).toLocaleDateString(),
        last_date: new Date(LastDate).toLocaleDateString(),
      }));
  } catch (error) {
    console.error("An error occurred while fetching the bar ranges:", error);
    return []; // Return an empty array or handle errors as needed
  }
};

const barRangesStore = writable(new Promise(() => []));

export const barRanges = {
  subscribe: barRangesStore.subscribe,
  load: async () => {
    const current = get(barRangesStore);
    if (current instanceof Promise) {
      const data = await fetchData();
      barRangesStore.set(data);
    }
  },
};

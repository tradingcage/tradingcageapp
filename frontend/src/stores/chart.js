import { writable, readable } from "svelte/store";
import { durations, indexSymbols } from "../util/constants.js";
import { appendBar } from "../util/bars.js";

export const lastPrices = writable({});

export const chartIsLoading = writable(false);

function createChartDataStore() {
  const { subscribe, set: setUnderlying } = writable({
    bars: [],
    meta: {
      timeframe: "5m",
    },
  });

  let currentBars = [];
  const set = (data) => {
    currentBars = data.bars;
    setUnderlying(data);
  };

  let abortController = null;

  const fetchFn = (meta) => {
    if (abortController) {
      abortController.abort();
    }
    abortController = new AbortController();

    const symbolID = indexSymbols[meta.index];
    chartIsLoading.set(true);

    fetch("/bars", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        SymbolID: symbolID,
        Timeframe: meta.timeframe,
        Duration: durations[meta.timeframe],
        EndDate: meta.enddate,
        Rth: meta.rth,
      }),
      signal: abortController.signal,
    })
      .then((response) => response.json())
      .then((data) => {
        const bars = data.bars;
        if (bars != null) {
          let newMeta = { ...meta };
          if (bars.length > 0) {
            const lastBar = bars[bars.length - 1];
            newMeta.enddate = new Date(lastBar.Date);
          }
          set({ bars, meta: newMeta });
        }
        if (data.lastPrices != null) {
          lastPrices.set(data.lastPrices);
        }
      })
      .catch((error) => {
        console.error(error);
      })
      .finally(() => {
        chartIsLoading.set(false);
      });
  };

  const manualUpdateFn = (meta, bar) => {
    appendBar(meta, currentBars, bar);
    set({ bars: currentBars, meta });

    lastPrices.update((prices) => {
      return { ...prices, [indexSymbols[meta.index]]: bar.Close };
    });
  };

  return {
    subscribe,
    fetch: fetchFn,
    manualUpdate: manualUpdateFn,
  };
}

export const chartData = createChartDataStore();

export const activeOrders = writable([]);

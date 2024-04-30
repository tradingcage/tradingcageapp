<script>
  import { onMount } from 'svelte';
  import FirChart from './components/FirChart.svelte';
  import LeftArrowCircle from './components/LeftArrowCircle.svelte';
  import PlayIcon from './components/PlayIcon.svelte';
  import PauseIcon from './components/PauseIcon.svelte';
  import InfoIcon from './components/InfoIcon.svelte';
  import BarRangesPopup from './components/BarRangesPopup.svelte';
  import { chartData } from './stores/chart.js';
  import { barRanges } from './stores/barRanges.js';
  import { toDatetimeLocal, fromDatetimeLocal } from './util/datetimeLocal.js';
  import { indexes, nonTradeable, timeframes, indexSymbols, humanReadableSymbol } from './util/constants.js';
  import { splitTimeframe, roundUpToTimeframe } from './util/bars.js';
  import { nextDay, prevDay } from './util/dates.js';

  const allIndexes = indexes.concat(nonTradeable);
  
  let chartMeta = {
    index: 'ES',
    timeframe: "5m",
    rth: true,
  };
  
  let barRangesPopupVisible = false;

  const skipAheadFrames = [
    "1m", "5m", "15m", "30m", "1h", "2h", "4h", "1d", "1w", "1m"
  ];
  
  let localEnddate;
  function updateLocalEnddate() {
     localEnddate = toDatetimeLocal(new Date(chartMeta.enddate));
  }
  
  let debounceTimer;
  function updateEnddate(event) {
    chartMeta.enddate = fromDatetimeLocal(localEnddate).getTime();
    clearTimeout(debounceTimer);
    updateChart();
  }

  function updateRTH() {
    updateChart();
  }
  function updateChart() {
    chartData.fetch(chartMeta);
  }

  function updateToPrevDay() {
    chartMeta.enddate = prevDay(chartMeta.enddate)
    updateLocalEnddate();
    updateChart();
  }

  function updateToNextDay() {
    chartMeta.enddate = nextDay(chartMeta.enddate)
    updateLocalEnddate();
    updateChart();
  }

  function updateChartEndDateWithLatest(ranges) {
    const symbolID = indexSymbols[chartMeta.index];
    if (!Array.isArray(ranges)) {
      return;
    }
    ranges.forEach(({ symbol_id, last_date }) => {
      if (symbol_id === symbolID) {
        chartMeta.enddate = new Date(last_date).getTime()
      }
    })
    updateLocalEnddate();
    updateChart();
  }

  onMount(async () => {
    await barRanges.load();
  });
  
  barRanges.subscribe(updateChartEndDateWithLatest);
</script>

<header class="flex-grow-0 flex-shrink-0 flex-auto flex-initial w-full p-4 bg-emerald-600 text-white">
  <div class="flex justify-between items-center">
    <div class="flex">
      <LeftArrowCircle classes="cursor-pointer" color="#ffffff" on:click={() => window.location.href = '/dashboard'} />
      <div class="text-lg ml-4">Chart Finder</div>
    </div>
    <div class="flex">
      <select bind:value={chartMeta.index} on:change={() => updateChart()} id="indexes-dropdown" class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        {#each allIndexes as index}
          <option value={index}>{humanReadableSymbol[index]}</option>
        {/each}
      </select>
      <select bind:value={chartMeta.timeframe} on:change={() => updateChart()} id="timeframes-dropdown" class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        {#each timeframes as timeframe}
          <option value={timeframe}>{timeframe}</option>
        {/each}
      </select>
      <select bind:value={chartMeta.rth} on:change={updateRTH} id="rth-dropdown" class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        <option value={true}>Regular Hours</option>
        <option value={false}>Extended Hours</option>
      </select>
      <button class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm" on:click={updateToPrevDay}>-1d</button>
      <input type="datetime-local" step="1" bind:value={localEnddate} on:input={updateEnddate} class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
      <button class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm" on:click={updateToNextDay}>+1d</button>
    </div>
  </div>
</header>

<div class="flex flex-row flex-grow overflow-y-auto">
  <FirChart />

  <div class="w-64 border bg-white rounded-lg shadow-lg ml-4 flex-shrink-0 overflow-y-auto">
    {#each Array(8) as _, i}
      <details class="bg-gray-100 rounded-lg">
        <summary class="px-4 py-2 text-lg font-semibold text-gray-700 cursor-pointer hover:bg-gray-200">Accordion Element {i + 1}</summary>
        <div class="pl-4 py-2 bg-white">
          {#each Array(25) as _, j}
            <p class="text-sm text-gray-600">Sub-element {j + 1}</p>
          {/each}
        </div>
      </details>
    {/each}
  </div>
</div>
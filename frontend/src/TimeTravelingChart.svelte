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
  import { nextDay, nextSunday, nextMonth } from './util/dates.js';

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

  let isPaused = true;

  let speedNumerator = "1s";
  let speedDenominator = 1;
  
  let ws;
  let wsActive = false;

  const sendPlayCommand = () => {
    const speedTimeframe = splitTimeframe(speedNumerator);
    const chartTimeframe = splitTimeframe(chartMeta.timeframe);
    ws.send(JSON.stringify({
      cmd: "play",
      frame: speedTimeframe,
      chartFrame: chartTimeframe,
      seconds: speedDenominator,
      rth: chartMeta.rth,
    }));
  };

  const sendPauseCommand = () => {
    ws.send(JSON.stringify({
      cmd: "pause"
    }));
  };
  
  const playPauseButtonPressed = () => {
    isPaused = !isPaused;
    if (!isPaused) {
      if (wsActive) {
        ws.close();
      }
      ws = new WebSocket(`wss://${window.location.hostname}/replay?startingDateMillis=${chartMeta.enddate}&symbolID=${indexSymbols[chartMeta.index]}`)
      ws.onopen = function (e) {
        wsActive = true;
        sendPlayCommand();
      };
      ws.onclose = function (e) {
        wsActive = false;
      };
      ws.onmessage = function (e) {
        const data = JSON.parse(e.data);
        if (data?.bars == null) 
        {
          return;
        }
        const barsData = data.bars[indexSymbols[chartMeta.index]];
        if (barsData.length > 0) {
          const bar = barsData[barsData.length - 1];
          chartMeta.enddate = bar.Date;
          updateLocalEnddate();
          for (let i = 0; i < barsData.length; i++) {
            if (barsData[i].Volume > 0) {
              chartData.manualUpdate(chartMeta, barsData[i]);
            }
          }
        }
      };
    } else if (wsActive) {
      sendPauseCommand();
    }
  }

  function updateSpeed() {
    if (wsActive && !isPaused) {
      sendPlayCommand();
    }
  }

  function updateRTH() {
    updateChart();
    if (wsActive && !isPaused) {
      sendPlayCommand();
    }
  }

  function pause() {
    if (wsActive && !isPaused) {
      isPaused = true;
      sendPauseCommand();
    }
  }

  function skipAhead(tf) {
    pause()
    const timeframe = splitTimeframe(tf);
    
    switch (timeframe.unit) {
      case 's':
        chartMeta.enddate += 1000 * timeframe.value;
        break;
      case 'm':
        chartMeta.enddate += 1000 * 60 * timeframe.value;
        break;
      case 'h':
        chartMeta.enddate += 1000 * 3600 * timeframe.value;
        break;
      case 'd':
        chartMeta.enddate = nextDay(chartMeta.enddate);
        break;
      case 'w':
        chartMeta.enddate = nextSunday(chartMeta.enddate);
        break;
      case 'mo':
        chartMeta.enddate = nextMonth(chartMeta.enddate);
        break;
      default:
        throw new Error(`Unknown timeframe: ${timeframe.unit}`);
    }
    updateLocalEnddate();
    updateEnddate();
  }

  onMount(async () => {
    await barRanges.load();
  });

  function updateChart() {
    pause()
    chartData.fetch(chartMeta);
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
  
  barRanges.subscribe(updateChartEndDateWithLatest);
</script>

<header class="flex-grow-0 flex-shrink-0 flex-auto flex-initial w-full p-4 bg-emerald-600 text-white">
  <div class="flex justify-between items-center">
    <div class="flex">
      <LeftArrowCircle classes="cursor-pointer" color="#ffffff" on:click={() => window.location.href = '/dashboard'} />
      <div class="text-lg ml-4">Time Traveling Chart</div>
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
      <input type="datetime-local" step="1" bind:value={localEnddate} on:input={updateEnddate} class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500" />
      <div class="flex justify-center items-center cursor-pointer" on:click={() => barRangesPopupVisible = true}>
        <InfoIcon color="#ffffff"/>
      </div>
    </div>
  </div>
</header>

<div class="flex flex-row flex-grow">
  <FirChart />
</div>

<footer class="flex-grow-0 flex-shrink-0 flex-auto flex-initial w-full bg-emerald-600 text-white p-4">
  <div class="flex justify-center items-center">
    <form class="flex space-x-2 items-center">
      <select id="replay-timeframe" bind:value={speedNumerator} on:change={updateSpeed} class="py-1 px-2 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        <option value="1s">1s</option>
        <option value="10s">10s</option>
        <option value="30s">30s</option>
        <option value="1m">1m</option>
        <option value="5m">5m</option>
        <option value="15m">15m</option>
        <option value="30m">30m</option>
        <option value="1h">1h</option>
        <option value="1d">1d</option>
      </select>
      <label for="replay-rate">per</label>
      <select id="replay-rate" bind:value={speedDenominator} on:change={updateSpeed} class="py-1 px-2 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        <option value={1}>1s</option>
        <option value={2}>2s</option>
        <option value={3}>3s</option>
        <option value={4}>4s</option>
        <option value={5}>5s</option>
      </select>
      <div class="pl-10"></div>
      <button type="button" class="flex items-center justify-center h-8 w-8 relative bg-transparent border border-white rounded-full p-1" on:click={playPauseButtonPressed}>
        {#if isPaused}
          <PlayIcon width="48px" height="48px" />
        {:else}
          <PauseIcon width="48px" height="48px" />
        {/if}
      </button>
      <div class="pl-8"></div>
      {#each skipAheadFrames as tf, i}
        {#if i >= skipAheadFrames.indexOf(chartMeta.timeframe) && i < skipAheadFrames.indexOf(chartMeta.timeframe) + 4}
          <div class={"cursor-pointer underline pl-1"} on:click={() => skipAhead(tf)}>
            +{tf}
          </div>
        {/if}
      {/each}
    </form>
  </div>
</footer>

<BarRangesPopup active={barRangesPopupVisible} closePopup={() => barRangesPopupVisible = false} />
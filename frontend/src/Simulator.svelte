<script>
  import FirChart from './components/FirChart.svelte';
  import RefreshIcon from './components/RefreshIcon.svelte';
  import PlayIcon from './components/PlayIcon.svelte';
  import PauseIcon from './components/PauseIcon.svelte';
  import LeftArrowCircle from './components/LeftArrowCircle.svelte';
  import HelpCircle from './components/HelpCircle.svelte';
  import { chartData, activeOrders, lastPrices } from './stores/chart.js';
  import { indexes, timeframes, indexSymbols, symbolsIndex, multipliers, minimumPriceFluctuations, humanReadableSymbol } from './util/constants.js';
  import { splitTimeframe } from './util/bars.js';
  import { toDatetimeLocal } from './util/datetimeLocal.js';

  // Page state
  
  let currentTab = 'orders';
  function setCurrentTab(tab) {
    isPriceInputFocused = false;
    currentTab = tab;
  }

  let { accountID, fulfilledOrders, positions } = globals;
  activeOrders.set(globals.activeOrders);

  let pnl = {
    realized: globals.realizedPnl,
    unrealized: 0,
  };

  let chartMeta = {
    index: 'ES',
    timeframe: "5m",
    rth: true,
    enddate: globals.date,
  };

  const skipAheadFrames = [
    "1m", "5m", "15m", "30m", "1h", "2h", "4h"
  ];

  let orderForm = {
    'type': 'market',
    'direction': 'buy',
    'price': null,
    'quantity': 1,
    'linkedOrders': [],
    'activateOnFill': true,
  };

  let isPriceInputFocused = false;
  let linkedOrderTooltipHovered = false;

  chartData.subscribe((data) => {
    if (data?.bars?.length > 0 && (!isPriceInputFocused || orderForm.type === 'market')) {
      orderForm.price = data.bars[data.bars.length - 1].Close;
    }
  });

  lastPrices.subscribe((lastPricesValues) => {
    pnl.unrealized = calculateUnrealizedPnl(lastPricesValues);
  })

  // Helper/summary functions
  
  function summarizeFulfilledOrder(order) {
    let summary = `${toDatetimeLocal(new Date(order.ActivatedAt))}: ${order.Direction === 'buy' ? 'Bought' : 'Sold'} ${order.OrderType} ${order.Quantity}x ${symbolsIndex[order.SymbolID]} @ ${order.FulfilledPrice}.`;
    return summary;
  }

  function summarizePosition(position) {
    return `${position.Direction === 'buy' ? 'Bought' : 'Sold'} ${position.Quantity}x ${symbolsIndex[position.SymbolID]} @ ${position.Price}.`;
  }
  
  function summarizeOrder(order) {
    let summary = `${toDatetimeLocal(new Date(order.ActivatedAt), true)}: ${order.Direction === 'buy' ? 'Buy' : 'Sell'} ${order.OrderType} ${order.Quantity}x ${symbolsIndex[order.SymbolID]}`;
    if (order.OrderType !== 'market') {
      summary += ` @ ${order.Price}`;
    }
    if (typeof order.EntryOrderID === 'number') {
      summary += ' (linked'
      if (!order.ActivatedAt) {
        summary += ', pending'
      }
      summary += ')';
    }
    summary += '.';
    return summary;
  }

  function calculateUnrealizedPnl(lastPricesValues) {
    let unrealizedPnl = 0;
    if (!Array.isArray(positions)) {
      return 0;
    }
    for (let position of positions) {
      const lastPrice = lastPricesValues[position.SymbolID];
      if (lastPrice == null || lastPrice == 0) {
        continue;
      }
      if (position.Direction === "buy") {
        unrealizedPnl += position.Quantity * (lastPrice - position.Price) * multipliers[position.SymbolID];
      } else {
        unrealizedPnl += position.Quantity * (position.Price - lastPrice) * multipliers[position.SymbolID];
      }
    }
    return unrealizedPnl;
  }

  function orderTitle(linkedOrder) {
    if (linkedOrder.direction !== orderForm.direction) {
      if (linkedOrder.type === 'limit') {
        return 'Take Profit';
      }
      if (linkedOrder.type === 'stop') {
        return 'Stop Loss';
      }
    }
    return 'Linked Order';
  }

  // Page actions

  function addLinkedOrder() {
    let priceMult = 1.005;
    let orderType = 'limit';
    if (orderForm.linkedOrders.length === 0) {
      orderForm.activatAfterFilled = false;
      orderType = 'stop';
      if (orderForm.direction === 'buy') {
        priceMult = 0.995;
      }
    } else if (orderForm.direction === 'sell') {
      priceMult = 0.995;
    }
    
    if (orderForm.linkedOrders.length < 5) {
      orderForm.linkedOrders = [...orderForm.linkedOrders, {
        'type': orderType,
        'direction': orderForm.direction === 'buy' ? 'sell' : 'buy',
        'price': typeof orderForm.price === 'number' ? 
          Math.ceil(orderForm.price * priceMult)
          : null,
        'quantity': orderForm.quantity,
      }];
    }
  }

  function removeLinkedOrder(index) {
    if (index >= 0 && index < orderForm.linkedOrders.length) {
      orderForm.linkedOrders = orderForm.linkedOrders.filter((_, i) => i !== index);
    }
  }
  
  function cancelOrder() {
    let orderID = parseInt(this.id.split('-')[1]);
    if (typeof orderID !== 'number') {
      return;
    }
    fetch("/cancel-order", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ accountID, orderID }),
    })
      .then((response) => response.json())
      .then((data) => {
        if (Array.isArray(data)) {
          activeOrders.set(data);
        }
      });
  }

  function submitOrder(e) {
    pause();
    let req = {
      accountID,
      symbolID: indexSymbols[chartMeta.index],
      entryOrder: {
        orderType: orderForm.type,
        direction: orderForm.direction,
        price: parseFloat(orderForm.price),
        quantity: parseInt(orderForm.quantity),
      },
      linkedOrders: orderForm.linkedOrders.map((linkedOrder) => ({
        orderType: linkedOrder.type,
        direction: linkedOrder.direction,
        price: parseFloat(linkedOrder.price),
        quantity: parseInt(linkedOrder.quantity),
        activateOnFill: orderForm.activateOnFill,
      })),
    };
    fetch('/submit-order', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(req)
      })
      .then(response => response.json())
      .then(data => {
        orderForm.linkedOrders = [];
        if (Array.isArray(data)) {
          activeOrders.set(data);
        }
      });
  }

  function updateAccountOrdersPositions(aop) {
    if (aop.account) {
      pnl.realized = aop.account.RealizedPnL;
      chartMeta.enddate = new Date(aop.account.Date).getTime();
    }
    if (aop.activeOrders || aop.fulfilledOrders || aop.positions) {
      activeOrders.set(aop.activeOrders ?? []);
      fulfilledOrders = aop.fulfilledOrders ?? [];
      positions = aop.positions ?? [];
    }
  }

  function incDate() {
    pause();
    let inc = this.id.split('-')[1];
    if (inc == null || inc.length == 0) {
      return;
    }
    fetch(
      "/inc-date",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ accountID, inc }),
      },
    )
      .then(response => response.json())
      .then(response => {
        updateAccountOrdersPositions(response);
        updateChart();
      });
  }

  // Sim actions

  let isPaused = true;
  let ws;
  let wsActive = false;
  let speedNumerator = "1s";
  let speedDenominator = 1;
  
  function updateSpeed() {
    if (wsActive && !isPaused) {
      sendPlayCommand();
    }
  }

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

  function playPauseButtonPressed() {
    isPaused = !isPaused;
    if (!isPaused) {
      if (wsActive) {
        ws.close();
      }
      ws = new WebSocket(`wss://${window.location.hostname}/simulate?accountID=${accountID}&symbolID=${indexSymbols[chartMeta.index]}`);
      ws.onopen = function (e) {
        wsActive = true;
        sendPlayCommand();
      };
      ws.onclose = function (e) {
        wsActive = false;
      };
      ws.onmessage = function (e) {
        const data = JSON.parse(e.data);
        if (data?.bars == null) {
          return;
        }
        updateAccountOrdersPositions(data);
        const barsData = data.bars[indexSymbols[chartMeta.index]];
        if (barsData.length > 0) {
          const bar = barsData[barsData.length - 1];
          chartMeta.enddate = bar.Date;
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

  function pause() {
    if (wsActive && !isPaused) {
      isPaused = true;
      sendPauseCommand();
    }
  }

  // Don't forget to keep the chart up to date
  
  function updateChart() {
    pause();
    chartData.fetch(chartMeta);
  }

  updateChart();
</script>

<header class="flex-grow-0 flex-shrink-0 flex-auto flex-initial w-full p-4 bg-blue-600 text-white">
  <div class="flex justify-between items-center">
    <div class="flex">
      <LeftArrowCircle classes="cursor-pointer" color="#ffffff" on:click={() => window.location.href = '/dashboard'} />
      <div class="text-lg ml-4">Replay Simulator</div>
    </div>
    <div class="flex">
      <select bind:value={chartMeta.index} on:change={updateChart} id="indexes-dropdown" class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        {#each indexes as index}
          <option value={index}>{humanReadableSymbol[index]}</option>
        {/each}
      </select>
      <select bind:value={chartMeta.timeframe} on:change={updateChart} id="timeframes-dropdown" class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        {#each timeframes as timeframe}
          <option value={timeframe}>{timeframe}</option>
        {/each}
      </select>
      <select bind:value={chartMeta.rth} on:change={updateChart} id="rth-dropdown" class="mr-2 py-2 px-3 bg-white text-black border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500">
        <option value={true}>Regular Hours</option>
        <option value={false}>Extended Hours</option>
      </select>
    </div>
  </div>
</header>

<div class="flex flex-row flex-grow overflow-y-auto">
  <FirChart />

  <div id="orders-pane" class='w-96 border bg-white rounded shadow-sm ml-4 flex-shrink-0 overflow-y-auto'>

    <div class="flex border-b">
      <button id="current-orders-button" class="flex-1 py-3 px-4 font-medium text-sm focus:outline-none hover:bg-gray-200 bg-gray-100" on:click={() => setCurrentTab('orders')}>Current Orders</button>
      <button id="trade-history-button" class="flex-1 py-3 px-4 font-medium text-sm focus:outline-none hover:bg-gray-200" on:click={() => setCurrentTab('history')}>Trade History</button>
    </div>

    <div id="current-orders-tab" class={`p-4 ${currentTab != 'orders' ? 'hidden' : ''}`}>

      <h2 class='text-lg font-bold mb-2'>Enter Order</h2>
      <form id="order-form" on:submit|preventDefault={submitOrder}>
        <div class="flex mb-4 ">
          <div class="w-1/2 px-2">
            <label class='block text-gray-700 text-sm font-bold mb-2' for='order-type'>Order Type</label>
            <select id='order-type' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' bind:value={orderForm.type}>
              <option value='market'>Market</option>
              <option value='limit'>Limit</option>
              <option value='stop'>Stop</option>
            </select>
          </div>
          <div class="w-1/2 px-2">
            <label class='block text-gray-700 text-sm font-bold mb-2' for='direction'>Direction</label>
            <select id='direction' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' bind:value={orderForm.direction}>
              <option value='buy'>Buy</option>
              <option value='sell'>Sell</option>
            </select>
          </div>
        </div>
        <div class="flex mb-4 ">
          <div class="w-1/2 px-2">
            <label class='block text-gray-700 text-sm font-bold mb-2' for='price'>Price</label>
            <input type='number' step={`${minimumPriceFluctuations[indexSymbols[chartMeta.index]]}`} id='price' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' disabled={orderForm.type === 'market'} bind:value={orderForm.price} on:focus={() => isPriceInputFocused = true}>
          </div>
          <div class="w-1/2 px-2">
            <label class='block text-gray-700 text-sm font-bold mb-2' for='quantity'>Quantity</label>
            <input type='number' step="1" id='quantity' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' bind:value={orderForm.quantity}>
          </div>
        </div>
        {#each orderForm.linkedOrders as linkedOrder, index (index)}
          <div class={`relative mb-2 rounded border-l border-r border-b ${orderForm.activateOnFill ? 'border-gray-500' : 'border-blue-500'}`}>
            <div class={`${orderForm.activateOnFill ? 'bg-gray-500' : 'bg-blue-500'} text-white text-sm font-bold flex justify-between items-center p-2 mt-2 rounded`}>
              <div>{orderTitle(linkedOrder)}</div>
              <div class="cursor-pointer" on:click={() => removeLinkedOrder(index)}>x</div>
            </div>
            <div class="flex mb-4 pt-2 ">
              <div class="w-1/2 px-2">
                <label class='block text-gray-700 text-sm font-bold mb-2' for='order-type'>Order Type</label>
                <select id='order-type' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' bind:value={linkedOrder.type}>
                  <option value='market'>Market</option>
                  <option value='limit'>Limit</option>
                  <option value='stop'>Stop</option>
                </select>
              </div>
              <div class="w-1/2 px-2">
                <label class='block text-gray-700 text-sm font-bold mb-2' for='direction'>Direction</label>
                <select id='direction' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' bind:value={linkedOrder.direction}>
                  <option value='buy'>Buy</option>
                  <option value='sell'>Sell</option>
                </select>
              </div>
            </div>
            <div class="flex mb-2 ">
              <div class="w-1/2 px-2">
                <label class='block text-gray-700 text-sm font-bold mb-2' for='price'>Price</label>
                <input type='number' step={`${minimumPriceFluctuations[indexSymbols[chartMeta.index]]}`} id='price' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' disabled={linkedOrder.type === 'market'} bind:value={linkedOrder.price} on:focus={() => isPriceInputFocused = true}>
              </div>
              <div class="w-1/2 px-2">
                <label class='block text-gray-700 text-sm font-bold mb-2' for='quantity'>Quantity</label>
                <input type='number' step="1" id='quantity' class='shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline' bind:value={linkedOrder.quantity}>
              </div>
            </div>
          </div>
        {/each}
        
        {#if orderForm.linkedOrders.length > 0}
          <label class="flex items-center mb-2 cursor-pointer relative">
            <input type="checkbox" class="form-checkbox" bind:checked={orderForm.activateOnFill}>
            <span class="ml-2 text-sm text-gray-700">Activate linked orders after entry fills</span>
            <div class="ml-2" on:mouseenter={() => linkedOrderTooltipHovered = true} on:mouseleave={() => linkedOrderTooltipHovered = false}>
              <HelpCircle width="18px" height="18px" />
              <div class={`absolute bottom-full w-1/2 right-2 bg-black text-white text-sm rounded p-1 ${!linkedOrderTooltipHovered ? 'hidden' : ''}`}>
                When checked, linked orders are only activated after the entry order is filled.
              </div>
            </div>
          </label>
        {/if}
        <button type="button" on:click={addLinkedOrder} class="flex-1 mb-2 text-sm font-bold cursor-pointer text-gray-500 hover:text-gray-600" disabled={orderForm.linkedOrders.length >= 5}>
          + Add Linked Order
        </button>
        <div class="border border-top border-gray-200 rounded mx-2 mb-2"></div>
        <button type='submit' class={`w-full text-white font-bold py-2 px-4 rounded focus:outline-none focus:shadow-outline ${orderForm.direction === 'buy' ? 'bg-emerald-500 hover:bg-emerald-700' : 'bg-red-500 hover:bg-red-700'}`}> 
          Submit {orderForm.linkedOrders.length > 0 ? 'OCO' : (orderForm.direction === 'buy' ? 'Buy' : 'Sell')} Order
        </button>
      </form>
      <div id="active-orders">
        <div class="mx-auto px-4 mt-4">
          <h2 class='text-lg font-bold mb-2'>Active Orders</h2>
          {#if $activeOrders.length > 0}
            {#each $activeOrders as order (order.ID)}
              <p class="text-sm font-medium text-gray-500"><span id={`order-${order.ID}`} class="text-blue-500 underline cursor-pointer cancel-order" on:click={cancelOrder}>[x]</span> {summarizeOrder(order)}</p>
            {/each}
          {:else}
          <p class='text-sm text-gray-500'>No active orders.</p>
          {/if}
        </div>
      </div>
      <div id="current-positions" class="mt-4">
        <div class="mx-auto px-4 mt-4">
          <h2 class='text-lg font-bold mb-2'>Positions</h2>
          {#if positions.length > 0}
            {#each positions as position}
            <p class="text-sm font-medium text-gray-500">{summarizePosition(position)}</p>
            {/each}
          {:else}
          <p class='text-sm text-gray-500'>No current positions.</p>
          {/if}
        </div>
      </div>
    </div>

    <div id="trade-history-tab" class="p-4 {`p-4 ${currentTab != 'history' ? 'hidden' : ''}`}">
      <h2 class='text-lg font-bold mb-2'>Trade History</h2>
      <div id="fulfilled-orders">
        {#if (fulfilledOrders.length > 0)}
          {#each fulfilledOrders as order}
            <p class="text-sm font-medium text-gray-500">{summarizeFulfilledOrder(order)}</p>
          {/each}
        {:else}
        <p class='text-sm text-gray-500'>No fulfilled orders.</p>
        {/if}
      </div>
    </div>

  </div>
</div>

<footer class="flex-grow-0 flex-shrink-0 flex-auto flex justify-between p-4 bg-gray-200 mt-4 w-full">
  <div class="flex">
    <p>Realized Account Value: <span id="realized-pnl" class="font-bold">${pnl.realized}</span></p>
    <p class="ml-4">Unrealized PnL: <span id="unrealized-pnl" class="font-bold">${pnl.unrealized}</span></p>
  </div>
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
    <button type="button" class="flex items-center justify-center h-8 w-8 relative bg-transparent border border-black rounded-full p-1" on:click={playPauseButtonPressed}>
      {#if isPaused}
        <PlayIcon width="48px" height="48px" color="#000000" />
      {:else}
        <PauseIcon width="48px" height="48px" color="#000000" />
      {/if}
    </button>
    <div class="pl-8"></div>
    {#each skipAheadFrames as tf, i}
      {#if i >= skipAheadFrames.indexOf(chartMeta.timeframe) - 1 && i < skipAheadFrames.indexOf(chartMeta.timeframe) + 4}
        <div id={`inc-${tf}`} class={"cursor-pointer underline pl-1"} on:click={incDate}>
          +{tf}
        </div>
      {/if}
    {/each}
    <div id="inc-next" class={"cursor-pointer underline pl-1"} on:click={incDate}>
      next open
    </div>
  </form>
  <time id="current-datetime" class="text-right">{toDatetimeLocal(new Date(chartMeta.enddate), true)}</time>
</footer>
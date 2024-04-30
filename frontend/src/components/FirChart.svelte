<div id="chart-container" class="w-full"></div>

<script>

import { chartData, activeOrders } from '../stores/chart.js';
import { indexSymbols } from '../util/constants.js';
import { shallowEqual } from '../util/stdlib.js';
  
const funcs = {
  refreshData: () => {},
  redrawOrders: () => {},
  refreshScaleExtent: () => {},
};

let oldMeta = {};
chartData.subscribe(data => {
  const forceRefresh = !shallowEqual(data?.meta, oldMeta, ["enddate"]);
  oldMeta = data?.meta;
  funcs.refreshData(data.bars, forceRefresh);
  if (data?.meta?.index == null) {
    return;
  }
  funcs.redrawOrders($activeOrders.filter(o => o.SymbolID === indexSymbols[data.meta.index]));
});

activeOrders.subscribe(orders => {
  if ($chartData?.meta?.index == null) {
    return;
  }
  funcs.redrawOrders(orders.filter(o => o.SymbolID === indexSymbols[$chartData.meta.index]));
});

function summarizeOrder(order) {
  return `${order.OrderType} ${order.Direction} ${order.Quantity}x` + (order.OrderType !== 'market' ? ` @ ${order.Price}` : '');
}
  
setTimeout(() => {
  const greenColor = "#449883";
  const redColor = "#db464a";
  const firChart = FirChart("chart-container", [], {
    colors: {
      bull: greenColor,
      bear: redColor,
    },
    persistIndicatorState: true,
    scaleExtent: [.2, 5],
    indicators: [
      "sma",
      "ema",
      "atr",
      "keltnerChannels",
      "bollingerBands",
      "rsi",
    ],
  });

  funcs.refreshScaleExtent = (timeframe) => {
    let scaleExtent = [.3, 5];
    if (timeframe === "15m") {
      scaleExtent[0] = .1;
    } else if (timeframe === "1h") {
      scaleExtent[0] = .025;
    }
    firChart.setScaleExtent(scaleExtent);
  };

  funcs.refreshData = (bars, forceRefresh) => {
    const truncatedBars = bars.slice(-500);
    firChart.refreshData(truncatedBars.map(({ Date: DateMillis, Open, High, Low, Close, Volume}) => ({
      date: new Date(DateMillis),
      open: Open,
      high: High,
      low: Low,
      close: Close,
      volume: Volume,
    })), forceRefresh);
  }

  let drawings = [];
  funcs.redrawOrders = (orders) => {
    drawings.forEach(drawing => drawing.remove());
    drawings = [];

    const bars = $chartData?.bars;
    if (!bars || bars.length < 2) {
      return;
    }

    const step = firChart.getMostCommonDifference();
    
    for (const order of orders) {
      const color = order.Direction === "buy" ? greenColor : redColor;
      const mostRecentDate = new Date(bars[bars.length - 1].Date + 2 * step);
      const drawing = firChart.addLineDrawing("", new Date(bars[0].Date), order.Price, mostRecentDate, order.Price, { color, hideFromInfoBox: true });
      const textDrawing = firChart.addTextDrawing(mostRecentDate, order.Price, summarizeOrder(order), "left");
      drawings.push(drawing);
      drawings.push(textDrawing);
    }
  };
});
  
</script>
import ChartFinder from "./ChartFinder.svelte";
import TimeTravelingChart from "./TimeTravelingChart.svelte";
import Simulator from "./Simulator.svelte";
import * as Sentry from "@sentry/svelte";

const chartElem = document.getElementById("chart");
const chartFinderElem = document.getElementById("chart-finder");
const simulatorElem = document.getElementById("simulator");

Sentry.init({
  dsn: "https://8fc1e75f528f95460815d4519bc8f64f@o4506507863851008.ingest.sentry.io/4506507869552640",
  integrations: [
    new Sentry.BrowserTracing({
      // Set 'tracePropagationTargets' to control for which URLs distributed tracing should be enabled
      tracePropagationTargets: [
        "localhost",
        /^https:\/\/tradingcage\.com/,
        /^https:\/\/.*\.replit\.dev/,
      ],
    }),
    new Sentry.Replay({
      maskAllText: false,
      blockAllMedia: false,
    }),
  ],
  // Performance Monitoring
  tracesSampleRate: 1.0, //  Capture 100% of the transactions
  // Session Replay
  replaysSessionSampleRate: 0.1, // This sets the sample rate at 10%. You may want to change it to 100% while in development and then sample at a lower rate in production.
  replaysOnErrorSampleRate: 1.0, // If you're not already sampling the entire session, change the sample rate to 100% when sampling sessions where errors occur.
});

let app;
if (chartElem != null) {
  app = new TimeTravelingChart({
    target: chartElem,
  });
} else if (chartFinderElem != null) {
  app = new ChartFinder({
    target: chartFinderElem,
  });
} else {
  app = new Simulator({
    target: simulatorElem,
  });
}

export default app;

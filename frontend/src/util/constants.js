export const indexes = [
  "ES",
  "NQ",
  "YM",
  "ER",
  "CL",
  "NG",
  "AD",
  "BP",
  "EC",
  "JY",
  "US",
  "FV",
  "TY",
];

export const nonTradeable = ["SP", "ND", "DJ"];

export const humanReadableSymbol = {
  ES: "S&P 500 E-mini (ES)",
  NQ: "Nasdaq 100 E-mini (NQ)",
  YM: "Dow Jones E-mini (YM)",
  ER: "Russell 2000 E-mini (RTY)",
  CL: "Crude Oil (CL)",
  NG: "Henry Hub Natural Gas (NG)",
  AD: "Australian Dollar (6A)",
  BP: "British Pound (6B)",
  EC: "Euro (6E)",
  JY: "Japanese Yen (6J)",
  US: "US Treasury Bonds (ZB)",
  TY: "10-Year T-Note (ZN)",
  FV: "5-Year T-Note (ZF)",
  SP: "S&P 500, original (SP)",
  ND: "Nasdaq 100, original (ND)",
  DJ: "Dow Jones, original (DJ)",
};

export const indexSymbols = {
  ES: 1,
  NQ: 2,
  YM: 3,
  AD: 14,
  BP: 15,
  CL: 16,
  DJ: 17,
  EC: 18,
  ER: 19,
  FV: 20,
  JY: 21,
  ND: 22,
  NG: 23,
  SP: 24,
  TY: 25,
  US: 26,
};

export const symbolsIndex = Object.fromEntries(
  Object.entries(indexSymbols).map(([symbol, index]) => [index, symbol]),
);

export const multipliers = {
  1: 50,
  2: 20,
  3: 5,
  14: 100000,
  15: 62500,
  16: 1000,
  18: 125000,
  19: 50,
  20: 100000,
  21: 12500000,
  23: 10000,
  25: 100000,
  26: 100000,
  17: 25,
  22: 100,
  24: 250,
};

export const minimumPriceFluctuations = {
  1: 0.25,
  2: 0.25,
  3: 1,
  14: 0.00005,
  15: 0.0001,
  16: 0.01,
  18: 0.00005,
  19: 0.1,
  20: 0.0078125,
  21: 0.0000005,
  23: 0.00025,
  25: 0.015625,
  26: 0.03125,
  17: 1,
  22: 0.25,
  24: 0.1,
};

export const timeframes = [
  "1s",
  "30s",
  "1m",
  "5m",
  "15m",
  "1h",
  "1d",
  "1w",
  "1mo",
];

export const durations = {
  "1s": "1d",
  "30s": "1d",
  "1m": "2d",
  "5m": "5d",
  "15m": "10d",
  "1h": "20d",
  "1d": "252d",
  "1w": "756d",
  "1mo": "2520d",
};

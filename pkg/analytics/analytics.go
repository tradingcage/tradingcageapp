package analytics

import (
	"sort"
)

// TradeMetrics holds calculated trade statistics.
type TradeMetrics struct {
	WinRate       float64
	ProfitFactor  float64
	LargestLoss   float64
	LargestProfit float64
	MedianLoss    float64
	MedianProfit  float64
}

// CalculateTradeMetrics calculates various metrics given an array of trades.
func CalculateTradeMetrics(trades []Trade) TradeMetrics {
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].ProfitOrLoss < trades[j].ProfitOrLoss
	})

	totalWins := 0.0
	totalLosses := 0.0
	totalProfit := 0.0
	totalLoss := 0.0
	profits := []float64{}
	losses := []float64{}

	for _, trade := range trades {
		if trade.ProfitOrLoss > 0 {
			totalWins++
			totalProfit += trade.ProfitOrLoss
			profits = append(profits, trade.ProfitOrLoss)
		} else {
			totalLosses++
			totalLoss += trade.ProfitOrLoss
			losses = append(losses, trade.ProfitOrLoss)
		}
	}

	numTrades := float64(len(trades))
	winRate := 0.0
	if numTrades > 0 {
		winRate = (totalWins / numTrades) * 100
	}

	profitFactor := 0.0
	if totalLoss != 0 {
		profitFactor = totalProfit / -totalLoss
	}

	medianProfit := calculateMedian(profits)
	medianLoss := calculateMedian(losses)

	var largestProfit, largestLoss float64
	if len(profits) > 0 {
		largestProfit = profits[len(profits)-1] // Last element after sorting
	}
	if len(losses) > 0 {
		largestLoss = losses[0] // First element after sorting
	}

	metrics := TradeMetrics{
		WinRate:       winRate,
		ProfitFactor:  profitFactor,
		LargestLoss:   largestLoss,
		LargestProfit: largestProfit,
		MedianLoss:    medianLoss,
		MedianProfit:  medianProfit,
	}

	return metrics
}

// calculateMedian calculates the median value of the given slice.
func calculateMedian(numbers []float64) float64 {
	numCount := len(numbers)
	if numCount == 0 {
		return 0.0
	}
	middle := numCount / 2
	if numCount%2 == 0 {
		return (numbers[middle-1] + numbers[middle]) / 2
	}
	return numbers[middle]
}

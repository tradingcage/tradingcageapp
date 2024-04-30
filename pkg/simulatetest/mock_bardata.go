package simulatetest

import (
	"github.com/tradingcage/tradingcage-go/pkg/bars"
)

// InMemoryBarData a mock for bars.BarData to supply predefined bars data for tests.
type InMemoryBarData struct {
	Bars map[uint][]bars.Bar // Map of symbolID to a slice of Bars
}

// NewInMemoryBarData creates a new InMemoryBarData with initialized data map.
func NewInMemoryBarData() *InMemoryBarData {
	return &InMemoryBarData{
		Bars: make(map[uint][]bars.Bar),
	}
}

// AddBars adds bars data for a specific symbolID.
func (m *InMemoryBarData) AddBars(symbolID uint, barsData []bars.Bar) {
	m.Bars[symbolID] = barsData
}

// GetBarsBetween fetches bars between given dates for a specific symbolID.
func (m *InMemoryBarData) GetBarsBetween(req bars.GetBarsBetweenRequest) ([]bars.Bar, error) {
	return m.Bars[req.SymbolID], nil
}

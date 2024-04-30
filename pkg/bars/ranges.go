package bars

import (
	"sync"
	"time"
)

// A structure that will be used to cache the query results
type SymbolDateRangeCache struct {
	Data      []SymbolDateRange
	Timestamp time.Time
}

var symbolDateRangeCache SymbolDateRangeCache
var cacheMutex sync.Mutex

// SymbolDateRange represents the result structure of our query.
type SymbolDateRange struct {
	SymbolID  int
	FirstDate time.Time
	LastDate  time.Time
}

func (td *timescaleData) GetSymbolDateRanges() ([]SymbolDateRange, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Check if the cache is fresh (less than an hour old)
	if time.Since(symbolDateRangeCache.Timestamp) < time.Hour {
		return symbolDateRangeCache.Data, nil
	}

	// Define query
	query := `
SELECT
    symbol_id,
    MIN(ts) AS first_date,
    MAX(ts) AS last_date
FROM
    ohlcv_daily
GROUP BY
    symbol_id;
`
	var results []SymbolDateRange

	// Execute the query
	rows, err := td.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var row SymbolDateRange
		if err := rows.Scan(&row.SymbolID, &row.FirstDate, &row.LastDate); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Update cache
	symbolDateRangeCache = SymbolDateRangeCache{
		Data:      results,
		Timestamp: time.Now(),
	}

	return results, nil
}

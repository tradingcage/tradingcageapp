package bars

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib" // Import the PostgreSQL Driver used by TimescaleDB
	"golang.org/x/sync/errgroup"
)

var (
	intervalForms = map[string]string{
		"s":  "second",
		"m":  "minute",
		"h":  "hour",
		"d":  "day",
		"w":  "week",
		"mo": "month",
	}

	timeFormat = "2006-01-02 15:04:05"

	locationChicago, _ = time.LoadLocation("America/Chicago")

	RTHTables = map[uint]string{
		1:  "ohlcv_daily_rth",
		2:  "ohlcv_daily_rth",
		3:  "ohlcv_daily_rth",
		14: "ohlcv_daily_rth_3",
		15: "ohlcv_daily_rth_3",
		16: "ohlcv_daily_rth_2",
		17: "ohlcv_daily_rth",
		18: "ohlcv_daily_rth_3",
		19: "ohlcv_daily_rth",
		20: "ohlcv_daily_rth_3",
		21: "ohlcv_daily_rth_3",
		22: "ohlcv_daily_rth",
		23: "ohlcv_daily_rth_2",
		24: "ohlcv_daily_rth",
		25: "ohlcv_daily_rth_3",
		26: "ohlcv_daily_rth_3",
	}

	rthClauses = map[string]string{
		"ohlcv_daily_rth": `
      AND (
          (EXTRACT(HOUR FROM %s) BETWEEN 9 AND 14) OR
          (EXTRACT(MINUTE FROM %s) > 30 AND EXTRACT(HOUR FROM %s) = 8) OR
          (EXTRACT(MINUTE FROM %s) <= 15 AND EXTRACT(HOUR FROM %s) = 15)
      )
    `,
		"ohlcv_daily_rth_2": `
      AND (
          (EXTRACT(HOUR FROM %s) BETWEEN 9 AND 12) OR
          (EXTRACT(MINUTE FROM %s) > 0 AND EXTRACT(HOUR FROM %s) = 8) OR
          (EXTRACT(MINUTE FROM %s) <= 30 AND EXTRACT(HOUR FROM %s) = 13)
      )
    `,
		"ohlcv_daily_rth_3": `
      AND (
          (EXTRACT(HOUR FROM %s) BETWEEN 8 AND 13) OR
          (EXTRACT(MINUTE FROM %s) > 20 AND EXTRACT(HOUR FROM %s) = 7) OR
          (EXTRACT(MINUTE FROM %s) = 0 AND EXTRACT(HOUR FROM %s) = 14)
      )
    `,
	}
)

func InvertedRTHTables() map[string][]uint {
	// Initialize the inverted map
	invertedTables := make(map[string][]uint)
	// Iterate through the original RTHTables map
	for symbolID, tableName := range RTHTables {
		// Check if the table name already exists in the inverted map
		if _, exists := invertedTables[tableName]; !exists {
			// If it doesn't exist, initialize an empty slice for this key
			invertedTables[tableName] = []uint{}
		}
		// Append the current symbol ID to the slice in the inverted map
		invertedTables[tableName] = append(invertedTables[tableName], symbolID)
	}
	return invertedTables
}

type timescaleData struct {
	db *sql.DB
}

// NewTimescaleData is a constructor function that initializes a timescaleData with the given connection string.
func NewTimescaleData(connectionString string) (*timescaleData, error) {
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		return nil, err
	}

	return &timescaleData{db: db}, nil
}

func (td *timescaleData) GetLastPrices(enddate int64, symbolID uint) (map[uint]float64, error) {
	end := time.UnixMilli(enddate).In(locationChicago).Format(timeFormat)

	query := `
    SELECT ts, symbol_id, close
    FROM ohlcv_1s
    WHERE ts <= $1::timestamp AT TIME ZONE 'UTC' AND ts > $1::timestamp at time zone 'UTC' - INTERVAL '1 days' AND symbol_id = $2
    GROUP BY ts, symbol_id, close
    ORDER BY ts DESC;
  `

	rows, err := td.db.Query(
		query,
		end,
		symbolID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not perform GetLastPrices sql query: %w", err)
	}
	defer rows.Close()

	lastPrices := make(map[uint]float64)

	for rows.Next() {
		var ts time.Time
		var symbolID sql.NullInt64
		var price sql.NullFloat64
		if err := rows.Scan(&ts, &symbolID, &price); err != nil {
			return nil, fmt.Errorf("could not scan rows: %w", err)
		}
		if !symbolID.Valid || !price.Valid {
			return lastPrices, nil
		}
		retSymbolID := uint(symbolID.Int64)
		if _, ok := lastPrices[retSymbolID]; !ok {
			lastPrices[retSymbolID] = price.Float64
		}
		if len(lastPrices) >= 3 {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not get rows: %w", err)
	}

	return lastPrices, nil
}

func (td *timescaleData) GetBarsBetween(req GetBarsBetweenRequest) ([]Bar, error) {
	timeframe, err := ParseTimeframe(req.Timeframe)
	if err != nil {
		return nil, err
	}

	// Construct all queries
	// One for every day but the last (if above intraday), one for the last day (will be empty if intraday)
	// A third for sub-minute granularity at the end if requested (both inter- and intra-day)

	mainQuery, err := getMainQuery(req.SymbolID, timeframe, req.RTH)
	if err != nil {
		return nil, fmt.Errorf("could not construct query: %w", err)
	}

	var endDateQuery string
	if timeframe.Unit == "d" || timeframe.Unit == "w" || timeframe.Unit == "mo" {
		endDateQuery = getEndDateQuery(req.RTH)
	}

	start := time.UnixMilli(req.StartDate).In(locationChicago).Format(timeFormat)
	end := time.UnixMilli(req.EndDate).In(locationChicago).Format(timeFormat)

	var endingSecondsQuery string
	if req.EndDate%60000 != 0 && timeframe.Unit != "s" {
		endingSecondsQuery = getEndingSecondsQuery(req.RTH)
	}

	// Execute both queries

	var eg errgroup.Group

	var mainBars []Bar
	var endDateBar *Bar
	var endingSecondsBar *Bar

	eg.Go(func() error {
		bars, err := td.doMainQuery(mainQuery, req.SymbolID, start, end)
		if err != nil {
			return fmt.Errorf("could not execute main query: %w", err)
		}
		mainBars = bars
		return nil
	})

	eg.Go(func() error {
		if endDateQuery == "" {
			return nil
		}
		bars, err := td.doSingleBarSingleTimeQuery(endDateQuery, req.SymbolID, end)
		if err != nil {
			return fmt.Errorf("could not execute endDate query: %w", err)
		}
		if len(bars) > 1 {
			return fmt.Errorf("returned %d bars, expected only 1", len(bars))
		}
		if len(bars) == 1 {
			endDateBar = &bars[0]
		}
		return nil
	})

	eg.Go(func() error {
		if endingSecondsQuery == "" {
			return nil
		}
		bars, err := td.doSingleBarSingleTimeQuery(endingSecondsQuery, req.SymbolID, end)
		if err != nil {
			return fmt.Errorf("could not execute endingSecondsQuery: %w", err)
		}
		if len(bars) > 1 {
			return fmt.Errorf("returned %d bars, expected only 1", len(bars))
		}
		if len(bars) == 1 {
			endingSecondsBar = &bars[0]
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	bars := mainBars

	// Combine the last day bar, if applicable
	if endDateBar != nil {
		if len(bars) == 0 {
			bars = append(bars, *endDateBar)
		} else {
			switch timeframe.Unit {
			case "d":
				bars = append(bars, *endDateBar)
			case "w":
				if IsSameWeek(bars[len(bars)-1].Date, endDateBar.Date) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endDateBar)
				} else {
					bars = append(bars, *endDateBar)
				}
			case "mo":
				if IsSameMonth(bars[len(bars)-1].Date, endDateBar.Date) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endDateBar)
				} else {
					bars = append(bars, *endDateBar)
				}
			default:
				return nil, fmt.Errorf("timeframe unit not recognized: %s", timeframe.Unit)
			}
		}
	}

	// Now combine the ending seconds bar, if applicable
	if endingSecondsBar != nil {
		if len(bars) == 0 {
			bars = append(bars, *endingSecondsBar)
		} else {
			switch timeframe.Unit {
			case "m":
				if IsSameGroupOfMinutes(bars[len(bars)-1].Date, endingSecondsBar.Date, timeframe.Value) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endingSecondsBar)
				} else {
					RoundUpBar(endingSecondsBar, timeframe)
					bars = append(bars, *endingSecondsBar)
				}
			case "h":
				if IsSameGroupOfHours(bars[len(bars)-1].Date, endingSecondsBar.Date, timeframe.Value) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endingSecondsBar)
				} else {
					RoundUpBar(endingSecondsBar, timeframe)
					bars = append(bars, *endingSecondsBar)
				}
			case "d":
				if IsSameDay(bars[len(bars)-1].Date, endingSecondsBar.Date) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endingSecondsBar)
				} else {
					bars = append(bars, *endingSecondsBar)
				}
			case "w":
				if IsSameWeek(bars[len(bars)-1].Date, endingSecondsBar.Date) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endingSecondsBar)
				} else {
					bars = append(bars, *endingSecondsBar)
				}
			case "mo":
				if IsSameMonth(bars[len(bars)-1].Date, endingSecondsBar.Date) {
					bars[len(bars)-1] = combineBars(bars[len(bars)-1], *endingSecondsBar)
				} else {
					bars = append(bars, *endingSecondsBar)
				}
			default:
				return nil, fmt.Errorf("timeframe unit not recognized: %s", timeframe.Unit)
			}
		}
	}

	return bars, nil
}

func (td *timescaleData) loggedQuery(query string, args ...interface{}) (*sql.Rows, error) {
	var params []string
	for _, arg := range args {
		params = append(params, fmt.Sprintf("%v", arg))
	}
	return td.db.Query(query, args...)
}

func (td *timescaleData) doMainQuery(
	mainQuery string,
	symbolID uint,
	start string,
	end string,
) ([]Bar, error) {
	rows, err := td.loggedQuery(
		mainQuery,
		symbolID,
		start,
		end,
	)
	if err != nil {
		return nil, fmt.Errorf("could not perform sql query: %w", err)
	}
	defer rows.Close()

	var bars []Bar
	for rows.Next() {
		var bar Bar
		var ts time.Time
		var Open, High, Low, Close, Volume sql.NullFloat64
		if err := rows.Scan(&ts, &Open, &High, &Low, &Close, &Volume); err != nil {
			return nil, fmt.Errorf("could not scan rows: %w", err)
		}
		if !Open.Valid || !High.Valid || !Low.Valid || !Close.Valid || !Volume.Valid {
			return bars, nil
		}
		adjTime, _ := time.ParseInLocation(timeFormat, ts.Format(timeFormat), locationChicago)
		bar.Date = adjTime.UnixMilli()
		bar.Open = Open.Float64
		bar.High = High.Float64
		bar.Low = Low.Float64
		bar.Close = Close.Float64
		bar.Volume = Volume.Float64
		bars = append(bars, bar)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not get rows: %w", err)
	}

	return bars, nil
}

func (td *timescaleData) doSingleBarSingleTimeQuery(
	query string,
	symbolID uint,
	end string,
) ([]Bar, error) {
	rows, err := td.loggedQuery(
		query,
		symbolID,
		end,
	)
	if err != nil {
		return nil, fmt.Errorf("could not perform sql query: %w", err)
	}
	defer rows.Close()

	var bars []Bar
	for rows.Next() {
		var bar Bar
		var ts time.Time
		var Open, High, Low, Close, Volume sql.NullFloat64
		if err := rows.Scan(&ts, &Open, &High, &Low, &Close, &Volume); err != nil {
			return nil, fmt.Errorf("could not scan rows: %w", err)
		}
		if !Open.Valid || !High.Valid || !Low.Valid || !Close.Valid || !Volume.Valid {
			return bars, nil
		}
		adjTime, _ := time.ParseInLocation(timeFormat, ts.Format(timeFormat), locationChicago)
		bar.Date = adjTime.UnixMilli()
		bar.Open = Open.Float64
		bar.High = High.Float64
		bar.Low = Low.Float64
		bar.Close = Close.Float64
		bar.Volume = Volume.Float64
		bars = append(bars, bar)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not get rows: %w", err)
	}

	return bars, nil
}

// GetBars retrieves bars data from TimescaleDB as per the provided GetBarsRequest.
func (td *timescaleData) GetBars(req GetBarsRequest) ([]Bar, error) {
	// parse the duration of trading days to int
	duration, err := strconv.ParseInt(req.Duration[:len(req.Duration)-1], 10, 64)
	if err != nil {
		return nil, err
	}

	// calculate the start and end times in epoch
	end := time.UnixMilli(req.EndDate)
	start := end.AddDate(0, 0, -int(duration+2))

	return td.GetBarsBetween(GetBarsBetweenRequest{
		SymbolID:  req.SymbolID,
		StartDate: start.UnixMilli(),
		EndDate:   end.UnixMilli(),
		Timeframe: req.Timeframe,
		RTH:       req.RTH,
	})
}

func getMainQuery(
	symbolID uint,
	timeframe Timeframe,
	rth bool,
) (string, error) {
	var tableName string
	var intraday bool
	var bucketFormat bool         // whether the table has the bucket/aggs schema or the ohlcv schema
	var intervalAdjustment string // adjustment to account for time_bucket using the beginning of the time window and TickData using the end of the time window as the time stamp for a candle
	switch timeframe.Unit {
	case "s":
		intraday = true
		tableName = "ohlcv_1s"
		intervalAdjustment = " - INTERVAL '1 second'"
	case "m", "h":
		intraday = true
		tableName = "ohlcv_1m"
		intervalAdjustment = " - INTERVAL '1 minute'"
	case "d", "w", "mo":
		if rth {
			tableName = RTHTables[symbolID]
			bucketFormat = true
		} else {
			tableName = "ohlcv_daily"
		}
	}

	if tableName == "" {
		return "", fmt.Errorf("did not recognize timeframe: %d %s", timeframe.Value, timeframe.Unit)
	}

	interval := fmt.Sprintf("%d %s", timeframe.Value, intervalForms[timeframe.Unit])
	var secondaryIntervalAdjustment string
	var endDateComparator string
	if intraday {
		secondaryIntervalAdjustment = fmt.Sprintf(" + INTERVAL '%s'", interval)
		// Include the end date on intraday
		endDateComparator = "<= $3"
	} else {
		// Ignore the last day if on daily and higher timeframes
		// It will be assembled out of 1 minute bars in a separate query and aggregated
		// at the application level
		endDateComparator = "< DATE_TRUNC('day', $3::timestamp) AT TIME ZONE 'UTC'"
	}

	query := `
    WITH aggs AS (
  `

	var bucketCol string
	if bucketFormat {
		bucketCol = fmt.Sprintf("time_bucket('%s'::interval, bucket %s)", interval, intervalAdjustment)
		query += fmt.Sprintf(`
        SELECT %s %s AS bucket, rollup(agg) AS agg
        FROM %s
        WHERE symbol_id = $1 AND bucket > $2 AND bucket %s
    `, bucketCol, secondaryIntervalAdjustment, tableName, endDateComparator)
	} else {
		bucketCol = fmt.Sprintf("time_bucket('%s'::interval, ts %s)", interval, intervalAdjustment)
		query += fmt.Sprintf(`
        SELECT %s %s AS bucket, rollup(candlestick(ts, open, high, low, close, volume)) AS agg
        FROM %s
        WHERE symbol_id = $1 AND ts > $2 AND ts %s
    `, bucketCol, secondaryIntervalAdjustment, tableName, endDateComparator)
	}
	if rth && intraday {
		var timeCol string
		if bucketFormat {
			timeCol = "bucket"
		} else {
			timeCol = "ts"
		}
		query += fmt.Sprintf(rthClauses[RTHTables[symbolID]], timeCol, timeCol, timeCol, timeCol, timeCol)
	}

	query += fmt.Sprintf(`
      GROUP BY %s
      ORDER BY %s ASC
    ), desc_aggs AS (
      SELECT bucket, open(agg) AS open, high(agg) AS high, low(agg) AS low, close(agg) AS close, volume(agg) AS volume
      FROM aggs
      ORDER BY bucket DESC
      LIMIT 5000
    )
    SELECT *
    FROM desc_aggs
    ORDER BY bucket ASC
  `, bucketCol, bucketCol)

	return query, nil
}

func getEndDateQuery(rth bool) string {
	// Assemble the last day out of 1 minute bars
	var timeAdjustment string
	if rth {
		timeAdjustment = ` + INTERVAL '8 hours 30 minutes'`
	} else {
		timeAdjustment = ` - INTERVAL '7 hours'`
	}
	return fmt.Sprintf(`WITH aggs AS (
    SELECT DATE_TRUNC('day', $2::timestamp) AS bucket, rollup(candlestick(ts, open, high, low, close, volume)) AS agg
    FROM ohlcv_1m
    WHERE symbol_id = $1 AND (
      ts > DATE_TRUNC('day', $2::timestamp) AT TIME ZONE 'UTC' %s
    ) AND ts <= $2
  )
  SELECT bucket, open(agg) AS open, high(agg) AS high, low(agg) AS low, close(agg) AS close, volume(agg) AS volume
  FROM aggs`, timeAdjustment)
}

func getEndingSecondsQuery(rth bool) string {
	var rthClause string
	if rth {
		rthClause = `AND (
      (EXTRACT(HOUR FROM ts) BETWEEN 9 AND 14) OR
      (EXTRACT(MINUTE FROM ts) > 30 AND EXTRACT(HOUR FROM ts) = 8) OR
      (EXTRACT(MINUTE FROM ts) <= 15 AND EXTRACT(HOUR FROM ts) = 15)
    )`
	}
	return fmt.Sprintf(`WITH aggs AS (
    SELECT TIME_BUCKET('1 minute'::interval, ts) + INTERVAL '1 minute' AS bucket, rollup(candlestick(ts, open, high, low, close, volume)) AS agg
    FROM ohlcv_1s
    WHERE ts >= DATE_TRUNC('minute', $2::timestamp) AT TIME ZONE 'UTC' AND ts <= $2 AND symbol_id = $1 %s
    GROUP BY bucket
  )
  SELECT bucket, open(agg) AS open, high(agg) AS high, low(agg) AS low, close(agg) AS close, volume(agg) AS volume
  FROM aggs`, rthClause)
}

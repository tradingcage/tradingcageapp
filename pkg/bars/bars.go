package bars

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"
)

var timeframeRegex = regexp.MustCompile(`^(\d+)([smhdw]|mo)$`)

type Timeframe struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"` // can be: s, m, h, d, w, mo
}

type GetBarsRequest struct {
	SymbolID  uint
	Timeframe string
	Duration  string
	RTH       bool
	EndDate   int64
}

type GetBarsBetweenRequest struct {
	SymbolID  uint
	Timeframe string
	StartDate int64
	EndDate   int64
	RTH       bool
}

type Bar struct {
	Date   int64
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type BarData interface {
	GetBars(req GetBarsRequest) ([]Bar, error)
	GetBarsBetween(req GetBarsBetweenRequest) ([]Bar, error)
	GetLastPrices(enddate int64, symbolID uint) (map[uint]float64, error)
	GetSymbolDateRanges() ([]SymbolDateRange, error)
}

func combineBars(bar1 Bar, bar2 Bar) Bar {
	return Bar{
		Date:   bar1.Date,
		Open:   bar1.Open,
		High:   math.Max(bar1.High, bar2.High),
		Low:    math.Min(bar1.Low, bar2.Low),
		Close:  bar2.Close,
		Volume: bar1.Volume + bar2.Volume,
	}
}

func (tf Timeframe) Millis() int64 {
	return TimeframeToDuration(tf).Milliseconds()
}

func (tf Timeframe) String() string {
	return fmt.Sprintf("%d%s", tf.Value, tf.Unit)
}

func (tf Timeframe) Empty() bool {
	return tf.Value == 0 || tf.Unit == ""
}

func IsSameWeek(t1, t2 int64) bool {
	// Convert to time.Time and local zero-time
	t1Time := time.UnixMilli(t1)
	t2Time := time.UnixMilli(t2)
	t1TimeZeroed := time.Date(t1Time.Year(), t1Time.Month(), t1Time.Day(), 0, 0, 0, 0, t1Time.Location())
	t2TimeZeroed := time.Date(t2Time.Year(), t2Time.Month(), t2Time.Day(), 0, 0, 0, 0, t2Time.Location())

	// Normalize to Sunday as start of the week
	t1Weekday := (int(t1TimeZeroed.Weekday()) + 6) % 7
	t2Weekday := (int(t2TimeZeroed.Weekday()) + 6) % 7

	// Start of the week times
	t1StartWeek := t1Time.AddDate(0, 0, -t1Weekday)
	t2StartWeek := t2Time.AddDate(0, 0, -t2Weekday)

	return t1StartWeek.Equal(t2StartWeek)
}

func IsSameMonth(t1, t2 int64) bool {
	// Convert int64 to time.Time
	t1Time := time.UnixMilli(t1)
	t2Time := time.UnixMilli(t2)

	return t1Time.Month() == t2Time.Month() && t1Time.Year() == t2Time.Year()
}

func IsSameGroupOfMinutes(t1, t2 int64, groupSize int) bool {
	t1Time := time.UnixMilli(t1).Add(-1 * time.Minute)
	t2Time := time.UnixMilli(t2).Add(-1 * time.Minute)
	return t1Time.Year() == t2Time.Year() &&
		t1Time.Month() == t2Time.Month() &&
		t1Time.Day() == t2Time.Day() &&
		t1Time.Hour() == t2Time.Hour() &&
		t1Time.Minute()/groupSize == t2Time.Minute()/groupSize
}

func IsSameGroupOfHours(t1, t2 int64, groupSize int) bool {
	t1Time := time.UnixMilli(t1).Add(-1 * time.Minute)
	t2Time := time.UnixMilli(t2).Add(-1 * time.Minute)
	return t1Time.Year() == t2Time.Year() &&
		t1Time.Month() == t2Time.Month() &&
		t1Time.Day() == t2Time.Day() &&
		t1Time.Hour()/groupSize == t2Time.Hour()/groupSize
}

func IsSameDay(t1, t2 int64) bool {
	t1Time := time.UnixMilli(t1)
	t2Time := time.UnixMilli(t2)
	return t1Time.Year() == t2Time.Year() &&
		t1Time.Month() == t2Time.Month() &&
		t1Time.Day() == t2Time.Day()
}

func DummyBar(date int64) Bar {
	return Bar{
		Date:   date,
		Volume: -1,
	}
}

func TimeframeToDuration(tf Timeframe) time.Duration {
	var duration time.Duration
	switch tf.Unit {
	case "s":
		duration = time.Duration(tf.Value) * time.Second
	case "m":
		duration = time.Duration(tf.Value) * time.Minute
	case "h":
		duration = time.Duration(tf.Value) * time.Hour
	case "d":
		duration = time.Duration(tf.Value) * 24 * time.Hour
	case "w":
		duration = time.Duration(tf.Value) * 7 * 24 * time.Hour
	case "mo":
		duration = time.Duration(tf.Value) * 30 * 24 * time.Hour
	default:
		return -1 * time.Minute
	}
	return duration
}

func RoundUpTime(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	rounded := t.Round(d)
	if rounded.Before(t) {
		rounded = rounded.Add(d)
	}
	return rounded
}

func RoundUpBar(bar *Bar, tf Timeframe) {
	bar.Date = RoundUpTime(time.UnixMilli(bar.Date), TimeframeToDuration(tf)).UnixMilli()
}

func ParseTimeframe(timeframe string) (Timeframe, error) {
	matches := timeframeRegex.FindStringSubmatch(timeframe)
	if len(matches) != 3 {
		return Timeframe{}, fmt.Errorf("invalid timeframe format: %s", timeframe)
	}
	timeframeValue, err := strconv.Atoi(matches[1])
	if err != nil {
		return Timeframe{}, fmt.Errorf("invalid timeframe value: %s", matches[1])
	}
	timeframeUnit := matches[2]
	return Timeframe{
		Value: timeframeValue,
		Unit:  timeframeUnit,
	}, nil
}

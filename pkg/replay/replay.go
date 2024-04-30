package replay

import (
	"fmt"
	"sync"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/bars"
)

type Replayer struct {
	sync.Mutex
	symbolIDs         []uint
	timeframe         bars.Timeframe
	chartFrame        bars.Timeframe
	rth               bool
	currentDateMillis int64
	barData           bars.BarData
	barCh             chan map[uint][]bars.Bar
	commsCh           chan Command
	closeCh           chan struct{}
	buffers           map[uint][]bars.Bar
	fetchThreshold    int
}

func NewReplayer(
	symbolIDs []uint,
	startingDateMillis int64,
	barData bars.BarData,
	barCh chan map[uint][]bars.Bar,
) *Replayer {

	r := &Replayer{
		symbolIDs:         symbolIDs,
		currentDateMillis: startingDateMillis,
		barData:           barData,
		barCh:             barCh,
		commsCh:           make(chan Command),
		closeCh:           make(chan struct{}),
		buffers:           make(map[uint][]bars.Bar),
		fetchThreshold:    100,
	}

	for _, symID := range symbolIDs {
		r.buffers[symID] = make([]bars.Bar, 0)
	}

	go r.runBackground()
	go r.fetchBarsInBackground()

	return r
}

func (r *Replayer) SendCommand(cmd Command) {
	r.commsCh <- cmd
}

func (r *Replayer) Play(frame bars.Timeframe, chartFrame bars.Timeframe, seconds int, rth bool) {
	r.commsCh <- NewCommand(
		"play",
		PlayCommand{
			Frame:      frame,
			ChartFrame: chartFrame,
			Seconds:    seconds,
			RTH:        rth,
		},
	)
}

func (r *Replayer) Pause() {
	r.commsCh <- NewCommand(
		"pause",
		PauseCommand{},
	)
}

func (r *Replayer) fetchBarsInBackground() {
	for {
		select {
		case <-r.closeCh:
			return
		default:
			r.Lock()
			timeframe := r.timeframe
			chartFrame := r.chartFrame
			r.Unlock()

			if len(r.commsCh) > 0 || timeframe.Empty() {
				time.Sleep(time.Second) // Avoid tight loop when paused
				continue
			}

			for _, symbolID := range r.symbolIDs {
				if len(r.buffers[symbolID]) > r.fetchThreshold {
					continue
				}
				r.refreshBuffer(symbolID, timeframe, chartFrame)
			}

			time.Sleep(3 * time.Second)
		}
	}
}

func (r *Replayer) refreshBuffer(symbolID uint, timeframe bars.Timeframe, chartFrame bars.Timeframe) {
	// Fetch new bars here and append to the buffer
	// Assuming fetching bars returns them in ascending date order
	var tf string
	if timeframe.Millis() < chartFrame.Millis() {
		tf = timeframe.String()
	} else {
		tf = chartFrame.String()
	}
	newBars, err := r.barData.GetBarsBetween(bars.GetBarsBetweenRequest{
		SymbolID:  symbolID,
		Timeframe: tf,
		StartDate: r.currentDateMillis,
		EndDate:   r.currentDateMillis + int64(r.fetchThreshold*int(timeframe.Millis())),
		RTH:       r.rth,
	})
	if err == nil {
		r.Lock()
		buffer := r.buffers[symbolID]
		if len(buffer) > 0 {
			lastBarDate := buffer[len(buffer)-1].Date
			for _, bar := range newBars {
				if bar.Date > lastBarDate {
					buffer = append(buffer, bar)
				}
			}
		} else {
			buffer = append(buffer, newBars...)
		}
		r.buffers[symbolID] = buffer
		r.Unlock()
	} else {
		fmt.Printf("error fetching bars in background for symbol %d: %s\n", symbolID, err)
	}
}

func (r *Replayer) runBackground() {
	paused := true
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-r.closeCh:
			return
		case cmd := <-r.commsCh:
			switch c := cmd.GetPayload().(type) {
			case PlayCommand:
				if err := c.Valid(); err != nil {
					fmt.Printf("play command not valid: %v\n", err)
					continue
				}
				paused = false
				r.Lock()
				r.chartFrame = c.ChartFrame
				r.timeframe = c.Frame
				for _, symbolID := range r.symbolIDs {
					r.buffers[symbolID] = make([]bars.Bar, 0)
				}
				r.rth = c.RTH
				r.Unlock()
				ticker.Reset(time.Duration(c.Seconds) * time.Second)
				for _, symbolID := range r.symbolIDs {
					r.refreshBuffer(symbolID, c.Frame, c.ChartFrame)
				}
			case PauseCommand:
				paused = true
				ticker.Stop()
			default:
				fmt.Printf("unknown command type: %v\n", c)
			}
		case <-ticker.C:
			r.Lock()
			if !paused {
				var barsToSend map[uint][]bars.Bar = make(map[uint][]bars.Bar)
				for _, symbolID := range r.symbolIDs {
					buffer := r.buffers[symbolID]
					var barsForSymbol []bars.Bar
					for _, bar := range buffer {
						if bar.Date <= r.currentDateMillis || bar.Date-r.currentDateMillis <= r.timeframe.Millis() {
							barsForSymbol = append(barsForSymbol, bar)
						} else {
							break
						}
					}

					if len(barsForSymbol) == 0 {
						barsForSymbol = append(barsForSymbol, bars.DummyBar(r.currentDateMillis+r.timeframe.Millis()))
					} else {
						r.buffers[symbolID] = buffer[len(barsForSymbol):]
					}

					barsToSend[symbolID] = barsForSymbol
				}

				r.barCh <- barsToSend
				r.currentDateMillis += r.timeframe.Millis()
			}
			r.Unlock()
		}
	}
}

func (r *Replayer) Close() {
	r.Pause()
	close(r.closeCh)
}

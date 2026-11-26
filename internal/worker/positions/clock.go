package positions

import "time"

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type Clock interface {
	NewTicker(interval time.Duration) Ticker
}

type systemClock struct{}

type systemTicker struct {
	ticker *time.Ticker
}

func NewSystemClock() Clock {
	return systemClock{}
}

func (systemClock) NewTicker(interval time.Duration) Ticker {
	return &systemTicker{ticker: time.NewTicker(interval)}
}

func (t *systemTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *systemTicker) Stop() {
	t.ticker.Stop()
}

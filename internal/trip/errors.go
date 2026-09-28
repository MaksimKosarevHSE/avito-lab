package trip

import "errors"

var (
	ErrInvalidTrip   = errors.New("invalid trip")
	ErrTripNotFound  = errors.New("trip not found")
	ErrTripCompleted = errors.New("trip already completed")
	ErrDriverBusy    = errors.New("driver already has an active trip")
)

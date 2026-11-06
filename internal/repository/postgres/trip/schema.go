package trip

// Constraints.
const activeTripPerDriverConstraint = "trips_one_active_per_driver_idx"

// Tables.
const (
	tripsTable                 = "trips"
	tripStatusHistoryTable     = "trip_status_history"
	tripCreateIdempotencyTable = "trip_create_idempotency"
)

// Columns.
const (
	idColumn             = "id"
	userIDColumn         = "user_id"
	driverIDColumn       = "driver_id"
	startLatitudeColumn  = "start_latitude"
	startLongitudeColumn = "start_longitude"
	endLatitudeColumn    = "end_latitude"
	endLongitudeColumn   = "end_longitude"
	priceColumn          = "price"
	statusColumn         = "status"
	startedAtColumn      = "started_at"
	finishedAtColumn     = "finished_at"
	createdAtColumn      = "created_at"
	updatedAtColumn      = "updated_at"

	tripIDColumn     = "trip_id"
	fromStatusColumn = "from_status"
	toStatusColumn   = "to_status"
	reasonColumn     = "reason"
	changedAtColumn  = "changed_at"

	idempotencyKeyColumn     = "idempotency_key"
	fingerprintVersionColumn = "fingerprint_version"
	requestFingerprintColumn = "request_fingerprint"
	expiresAtColumn          = "expires_at"
)

var tripColumns = []string{
	idColumn,
	userIDColumn,
	driverIDColumn,
	startLatitudeColumn,
	startLongitudeColumn,
	endLatitudeColumn,
	endLongitudeColumn,
	priceColumn,
	statusColumn,
	startedAtColumn,
	finishedAtColumn,
	createdAtColumn,
	updatedAtColumn,
}

var statusHistoryColumns = []string{
	tripIDColumn,
	fromStatusColumn,
	toStatusColumn,
	reasonColumn,
	changedAtColumn,
}

var idempotencyColumns = []string{
	idempotencyKeyColumn,
	fingerprintVersionColumn,
	requestFingerprintColumn,
	tripIDColumn,
	createdAtColumn,
	expiresAtColumn,
}

var idempotencyResultColumns = []string{
	idempotencyKeyColumn,
	fingerprintVersionColumn,
	requestFingerprintColumn,
	tripIDColumn,
}

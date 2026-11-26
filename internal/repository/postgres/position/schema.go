package position

const positionsTable = "trip_positions"

const (
	idColumn         = "id"
	tripIDColumn     = "trip_id"
	latitudeColumn   = "latitude"
	longitudeColumn  = "longitude"
	recordedAtColumn = "recorded_at"
	createdAtColumn  = "created_at"
)

var positionInsertColumns = []string{
	tripIDColumn,
	latitudeColumn,
	longitudeColumn,
	recordedAtColumn,
	createdAtColumn,
}

var positionColumns = []string{
	idColumn,
	tripIDColumn,
	latitudeColumn,
	longitudeColumn,
	recordedAtColumn,
	createdAtColumn,
}

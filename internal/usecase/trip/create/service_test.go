package create

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateFingerprint_CreatesEqualFingerprintForEqualInputs(t *testing.T) {
	input := CreateInput{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  50,
		StartLongitude: 50,
		EndLatitude:    50,
		EndLongitude:   50,
		Price:          200,
	}
	eqInput := input

	firstResult := createInputFingerprint(input)
	secondResult := createInputFingerprint(eqInput)
	assert.Len(t, firstResult, sha256.Size)
	assert.Len(t, secondResult, sha256.Size)
	assert.Equal(t, firstResult, secondResult)
}

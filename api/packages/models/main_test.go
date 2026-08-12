package models_test

import (
	"testing"

	"vitract-assessment/packages/dbtest"
)

func TestAutoMigrate(t *testing.T) {
	dbtest.New(t)
}

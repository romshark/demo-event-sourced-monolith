package testdb_test

import (
	"log/slog"
	"testing"

	"github.com/romshark/demo-event-sourced-monolith/database/testdb"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	db := testdb.New(t, t.Context(), slog.Default())
	row := db.QueryRow(t.Context(), `SELECT '1';`)
	var val string
	err := row.Scan(&val)
	require.NoError(t, err)
	require.Equal(t, "1", val)
}

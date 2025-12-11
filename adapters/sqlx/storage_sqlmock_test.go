package sqlx_test

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	libsqlx "github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	storage "gamifykit/adapters/sqlx"
	"gamifykit/core"
)

func newMockStore(t *testing.T) (*storage.Store, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	xdb := storage.NewWithDB(libsqlx.NewDb(db, "postgres"), storage.DriverPostgres)
	cleanup := func() {
		_ = db.Close()
	}
	return xdb, mock, cleanup
}

func TestSQLMock_AddPoints_Insert(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	ctx := context.Background()
	user := core.UserID("u1")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT points FROM user_points`).
		WithArgs(user, core.MetricXP).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO user_points`).
		WithArgs(user, core.MetricXP, int64(10), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	total, err := store.AddPoints(ctx, user, core.MetricXP, 10)
	require.NoError(t, err)
	require.Equal(t, int64(10), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLMock_AwardBadge_Insert(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	ctx := context.Background()
	user := core.UserID("u1")
	badge := core.Badge("b1")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(user, badge).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`INSERT INTO user_badges`).
		WithArgs(user, badge, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, store.AwardBadge(ctx, user, badge))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLMock_GetState(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	ctx := context.Background()
	user := core.UserID("u1")

	mock.ExpectQuery(`SELECT metric, points FROM user_points`).
		WithArgs(user).
		WillReturnRows(sqlmock.NewRows([]string{"metric", "points"}).
			AddRow("xp", 50).
			AddRow("points", 20))

	mock.ExpectQuery(`SELECT badge FROM user_badges`).
		WithArgs(user).
		WillReturnRows(sqlmock.NewRows([]string{"badge"}).AddRow("onboarded"))

	mock.ExpectQuery(`SELECT metric, level FROM user_levels`).
		WithArgs(user).
		WillReturnRows(sqlmock.NewRows([]string{"metric", "level"}).AddRow("xp", 3))

	state, err := store.GetState(ctx, user)
	require.NoError(t, err)
	require.Equal(t, int64(50), state.Points[core.MetricXP])
	require.Equal(t, int64(20), state.Points[core.MetricPoints])
	require.Contains(t, state.Badges, core.Badge("onboarded"))
	require.Equal(t, int64(3), state.Levels[core.MetricXP])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLMock_SetLevel_Insert(t *testing.T) {
	store, mock, cleanup := newMockStore(t)
	defer cleanup()

	ctx := context.Background()
	user := core.UserID("u1")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(user, core.MetricXP).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`INSERT INTO user_levels`).
		WithArgs(user, core.MetricXP, int64(2), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, store.SetLevel(ctx, user, core.MetricXP, 2))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLMock_AddPoints_ZeroDelta(t *testing.T) {
	store, _, cleanup := newMockStore(t)
	defer cleanup()

	_, err := store.AddPoints(context.Background(), "u1", core.MetricXP, 0)
	require.Error(t, err)
}

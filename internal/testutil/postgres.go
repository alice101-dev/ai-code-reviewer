package testutil

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
)

const databaseURLEnv = "TEST_DATABASE_URL"

func NewPostgres(t testing.TB) *gorm.DB {
	t.Helper()

	dsn := os.Getenv(databaseURLEnv)
	if dsn == "" {
		t.Skipf("%s is not set", databaseURLEnv)
	}

	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedSchema := pgx.Identifier{schema}.Sanitize()

	admin := open(t, dsn)
	require.NoError(t, admin.Exec("CREATE SCHEMA "+quotedSchema).Error)
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + quotedSchema + " CASCADE")
		_ = database.Close(admin)
	})

	db := open(t, withSearchPath(t, dsn, schema))
	t.Cleanup(func() {
		_ = database.Close(db)
	})

	for _, file := range migrationFiles(t) {
		statements, err := os.ReadFile(filepath.Clean(file))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(statements)).Error, "apply migration %s", filepath.Base(file))
	}

	return db
}

func open(t testing.TB, dsn string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	require.NoError(t, err)
	return db
}

func withSearchPath(t testing.TB, dsn, schema string) string {
	t.Helper()

	u, err := url.Parse(dsn)
	require.NoError(t, err)

	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	return u.String()
}

func migrationFiles(t testing.TB) []string {
	t.Helper()

	_, current, _, ok := runtime.Caller(0)
	require.True(t, ok)

	dir := filepath.Join(filepath.Dir(current), "..", "..", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	sort.Strings(files)
	return files
}

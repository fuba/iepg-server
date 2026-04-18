package db

import (
	"database/sql"
	"testing"

	"github.com/fuba/iepg-server/models"
	_ "github.com/mattn/go-sqlite3"
)

// legacySchema re-creates the pre-migration programs table (no networkId column)
// so we can validate the ALTER TABLE + backfill path.
const legacySchema = `
CREATE TABLE programs (
	id            INTEGER PRIMARY KEY,
	serviceId     INTEGER,
	startAt       INTEGER,
	duration      INTEGER,
	name          TEXT,
	description   TEXT,
	nameForSearch TEXT,
	descForSearch TEXT,
	seriesId      INTEGER,
	seriesEpisode INTEGER,
	seriesLastEpisode INTEGER,
	seriesName    TEXT,
	seriesRepeat  INTEGER,
	seriesPattern INTEGER,
	seriesExpiresAt INTEGER
);
`

// seedLegacyRow inserts a row that pre-dates the networkId migration.
func seedLegacyRow(t *testing.T, db *sql.DB, id, serviceId int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO programs (id, serviceId, startAt, duration, name) VALUES (?, ?, 0, 0, '')`, id, serviceId); err != nil {
		t.Fatalf("seed id=%d: %v", id, err)
	}
}

func TestMigration_BackfillNetworkID(t *testing.T) {
	path := t.TempDir() + "/legacy.db"
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	// Mirakurun format: id = networkId * 10^10 + serviceId * 10^5 + eventId
	mkID := func(net, svc, ev int64) int64 { return net*10000000000 + svc*100000 + ev }

	// Well-formed Mirakurun rows (should be backfilled)
	seedLegacyRow(t, raw, mkID(32742, 1072, 51879), 1072) // テレ東
	seedLegacyRow(t, raw, mkID(7, 312, 63155), 312)       // CS

	// Malformed row: id big enough but serviceId doesn't match encoded id (should NOT be backfilled)
	seedLegacyRow(t, raw, mkID(32742, 1072, 1), 999)

	// Small id (< 10^10): legacy fake id (should NOT be backfilled)
	seedLegacyRow(t, raw, 12345, 101)
	raw.Close()

	// Now run InitDB against the same file — it must migrate + backfill safely.
	db, err := InitDB(path)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	cases := []struct {
		id   int64
		want int64
	}{
		{mkID(32742, 1072, 51879), 32742},
		{mkID(7, 312, 63155), 7},
		{mkID(32742, 1072, 1), 0}, // serviceId mismatch → not backfilled
		{12345, 0},                // too small → not backfilled
	}
	for _, c := range cases {
		var got int64
		if err := db.QueryRow("SELECT networkId FROM programs WHERE id = ?", c.id).Scan(&got); err != nil {
			t.Fatalf("query id=%d: %v", c.id, err)
		}
		if got != c.want {
			t.Errorf("id=%d networkId=%d want %d", c.id, got, c.want)
		}
	}
}

func TestMigration_IsIdempotent(t *testing.T) {
	path := t.TempDir() + "/idem.db"

	// Run InitDB twice — neither call should error.
	db1, err := InitDB(path)
	if err != nil {
		t.Fatalf("first InitDB: %v", err)
	}
	db1.Close()

	db2, err := InitDB(path)
	if err != nil {
		t.Fatalf("second InitDB: %v", err)
	}
	defer db2.Close()

	ok, err := hasColumn(db2, "programs", "networkId")
	if err != nil || !ok {
		t.Fatalf("networkId column missing after idempotent init: ok=%v err=%v", ok, err)
	}
}

func TestSearchProgramsScansNetworkID(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Insert with networkId explicitly set via new INSERT path.
	if _, err := db.Exec(`INSERT INTO programs (id, serviceId, networkId, startAt, duration, name, description, nameForSearch, descForSearch, seriesId, seriesEpisode, seriesLastEpisode, seriesName, seriesRepeat, seriesPattern, seriesExpiresAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		327420107251879, 1072, 32742, 1, 1, "x", "", models.NormalizeForSearch("x"), "", nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}

	programs, err := SearchPrograms(db, "", 1072, 0, 0, 0)
	if err != nil {
		t.Fatalf("SearchPrograms: %v", err)
	}
	if len(programs) != 1 {
		t.Fatalf("got %d programs, want 1", len(programs))
	}
	if programs[0].NetworkID != 32742 {
		t.Errorf("NetworkID=%d want 32742", programs[0].NetworkID)
	}

	got, err := GetProgramByID(db, 327420107251879)
	if err != nil {
		t.Fatalf("GetProgramByID: %v", err)
	}
	if got.NetworkID != 32742 {
		t.Errorf("GetProgramByID NetworkID=%d want 32742", got.NetworkID)
	}
}

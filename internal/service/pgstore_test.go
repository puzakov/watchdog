package service

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	models "github.com/puzakov/watchdog/internal/model"
)

// pgDSN returns the PostgreSQL connection string for tests, or empty string
// if PG tests are disabled. Set TEST_PG_DSN environment variable to enable.
//
//	docker-compose up -d postgres
//	export TEST_PG_DSN="postgres://watchdog_db_user:secret@localhost:5434/watchdog_db_app"
//	go test -v -run TestPG ./internal/service/
func pgDSN() string {
	return os.Getenv("TEST_PG_DSN")
}

// newTestPGStore creates a PostgresStorage for testing.
// It connects using TEST_PG_DSN, truncates all data, and returns the store
// along with a cleanup function that closes the pool.
func newTestPGStore(t *testing.T) (*PostgresStorage, func()) {
	t.Helper()

	dsn := pgDSN()
	if dsn == "" {
		t.Skip("set TEST_PG_DSN to run PostgreSQL tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}

	// Clean any leftover data from previous test runs.
	if _, err := pool.Exec(ctx, `TRUNCATE metrics`); err != nil {
		pool.Close()
		t.Fatalf("truncate metrics: %v", err)
	}

	store := &PostgresStorage{pool: pool}
	cleanup := func() {
		_, _ = pool.Exec(ctx, `TRUNCATE metrics`)
		pool.Close()
	}

	return store, cleanup
}

func TestPG_NewPostgresStorage(t *testing.T) {
	dsn := pgDSN()
	if dsn == "" {
		t.Skip("set TEST_PG_DSN to run PostgreSQL tests")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	s := NewPostgresStorage(pool)
	if s == nil {
		t.Fatal("NewPostgresStorage returned nil")
	}
}

func TestPG_UpdateAndGetGauge(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	// Update a gauge metric.
	if err := store.UpdateGauge(ctx, "Alloc", 42.5); err != nil {
		t.Fatal("UpdateGauge:", err)
	}

	// Read it back.
	val, ok := store.GetGauge(ctx, "Alloc")
	if !ok {
		t.Fatal("GetGauge returned !ok")
	}
	if val != 42.5 {
		t.Fatalf("GetGauge = %f, want 42.5", val)
	}

	// Update again (overwrite).
	if err := store.UpdateGauge(ctx, "Alloc", 100.0); err != nil {
		t.Fatal("UpdateGauge second:", err)
	}
	val, ok = store.GetGauge(ctx, "Alloc")
	if !ok || val != 100.0 {
		t.Fatalf("GetGauge after second update = %f, want 100.0", val)
	}
}

func TestPG_UpdateAndGetCounter(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	// Update a counter metric.
	if err := store.UpdateCounter(ctx, "PollCount", 10); err != nil {
		t.Fatal("UpdateCounter:", err)
	}

	// Read it back.
	val, ok := store.GetCounter(ctx, "PollCount")
	if !ok {
		t.Fatal("GetCounter returned !ok")
	}
	if val != 10 {
		t.Fatalf("GetCounter = %d, want 10", val)
	}

	// Update again (accumulate).
	if err := store.UpdateCounter(ctx, "PollCount", 5); err != nil {
		t.Fatal("UpdateCounter second:", err)
	}
	val, ok = store.GetCounter(ctx, "PollCount")
	if !ok || val != 15 {
		t.Fatalf("GetCounter after accumulation = %d, want 15", val)
	}
}

func TestPG_GetGauge_NotFound(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	_, ok := store.GetGauge(context.Background(), "nonexistent")
	if ok {
		t.Fatal("expected !ok for nonexistent gauge")
	}
}

func TestPG_GetCounter_NotFound(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	_, ok := store.GetCounter(context.Background(), "nonexistent")
	if ok {
		t.Fatal("expected !ok for nonexistent counter")
	}
}

func TestPG_UpdateBatch(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	v1 := 1.5
	v2 := 2.5
	d1 := int64(100)
	d2 := int64(200)

	batch := []models.Metrics{
		{ID: "Gauge1", MType: models.Gauge, Value: &v1},
		{ID: "Gauge2", MType: models.Gauge, Value: &v2},
		{ID: "Counter1", MType: models.Counter, Delta: &d1},
		{ID: "Counter2", MType: models.Counter, Delta: &d2},
	}

	if err := store.UpdateBatch(ctx, batch); err != nil {
		t.Fatal("UpdateBatch:", err)
	}

	// Verify all metrics.
	gv, ok := store.GetGauge(ctx, "Gauge1")
	if !ok || gv != 1.5 {
		t.Fatalf("Gauge1 = %f, want 1.5", gv)
	}
	gv, ok = store.GetGauge(ctx, "Gauge2")
	if !ok || gv != 2.5 {
		t.Fatalf("Gauge2 = %f, want 2.5", gv)
	}
	cv, ok := store.GetCounter(ctx, "Counter1")
	if !ok || cv != 100 {
		t.Fatalf("Counter1 = %d, want 100", cv)
	}
	cv, ok = store.GetCounter(ctx, "Counter2")
	if !ok || cv != 200 {
		t.Fatalf("Counter2 = %d, want 200", cv)
	}
}

func TestPG_UpdateBatch_DedupGauge(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	v1 := 1.0
	v2 := 2.0

	batch := []models.Metrics{
		{ID: "Same", MType: models.Gauge, Value: &v1},
		{ID: "Same", MType: models.Gauge, Value: &v2},
	}

	if err := store.UpdateBatch(ctx, batch); err != nil {
		t.Fatal("UpdateBatch:", err)
	}

	// Last value should win.
	gv, ok := store.GetGauge(ctx, "Same")
	if !ok || gv != 2.0 {
		t.Fatalf("Same gauge = %f, want 2.0 (last wins)", gv)
	}
}

func TestPG_UpdateBatch_DedupCounter(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	d1 := int64(10)
	d2 := int64(20)

	batch := []models.Metrics{
		{ID: "Same", MType: models.Counter, Delta: &d1},
		{ID: "Same", MType: models.Counter, Delta: &d2},
	}

	if err := store.UpdateBatch(ctx, batch); err != nil {
		t.Fatal("UpdateBatch:", err)
	}

	// Deltas should be summed.
	cv, ok := store.GetCounter(ctx, "Same")
	if !ok || cv != 30 {
		t.Fatalf("Same counter = %d, want 30 (sum)", cv)
	}
}

func TestPG_Snapshot(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	// Insert some data.
	_ = store.UpdateGauge(ctx, "G1", 10.0)
	_ = store.UpdateGauge(ctx, "G2", 20.0)
	_ = store.UpdateCounter(ctx, "C1", 5)
	_ = store.UpdateCounter(ctx, "C2", 15)

	gauges, counters := store.Snapshot(ctx)

	if len(gauges) != 2 {
		t.Fatalf("expected 2 gauges, got %d", len(gauges))
	}
	if len(counters) != 2 {
		t.Fatalf("expected 2 counters, got %d", len(counters))
	}

	if gauges["G1"] != 10.0 {
		t.Errorf("G1 = %f, want 10.0", gauges["G1"])
	}
	if gauges["G2"] != 20.0 {
		t.Errorf("G2 = %f, want 20.0", gauges["G2"])
	}
	if counters["C1"] != 5 {
		t.Errorf("C1 = %d, want 5", counters["C1"])
	}
	if counters["C2"] != 15 {
		t.Errorf("C2 = %d, want 15", counters["C2"])
	}
}

func TestPG_NilReceiver(t *testing.T) {
	var s *PostgresStorage

	// All methods should handle nil receiver gracefully.
	ctx := context.Background()

	if err := s.UpdateGauge(ctx, "x", 1); err != nil {
		t.Error("UpdateGauge on nil:", err)
	}
	if err := s.UpdateCounter(ctx, "x", 1); err != nil {
		t.Error("UpdateCounter on nil:", err)
	}
	if err := s.UpdateBatch(ctx, nil); err != nil {
		t.Error("UpdateBatch on nil:", err)
	}

	_, ok := s.GetGauge(ctx, "x")
	if ok {
		t.Error("GetGauge on nil returned ok")
	}
	_, ok = s.GetCounter(ctx, "x")
	if ok {
		t.Error("GetCounter on nil returned ok")
	}

	g, c := s.Snapshot(ctx)
	if g == nil || c == nil {
		t.Error("Snapshot on nil returned nil maps")
	}
}

func TestPG_UpdateBatch_Empty(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	if err := store.UpdateBatch(context.Background(), nil); err != nil {
		t.Fatal("UpdateBatch with nil:", err)
	}
	if err := store.UpdateBatch(context.Background(), []models.Metrics{}); err != nil {
		t.Fatal("UpdateBatch with empty:", err)
	}
}

func TestPG_MultipleGaugesAndCounters(t *testing.T) {
	store, cleanup := newTestPGStore(t)
	defer cleanup()

	ctx := context.Background()

	// Insert multiple values sequentially.
	for i := range 10 {
		name := "Gauge" + string(rune('0'+i))
		if err := store.UpdateGauge(ctx, name, float64(i)*1.5); err != nil {
			t.Fatal(err)
		}
	}

	// Verify all.
	for i := range 10 {
		name := "Gauge" + string(rune('0'+i))
		val, ok := store.GetGauge(ctx, name)
		if !ok {
			t.Fatalf("gauge %s not found", name)
		}
		if val != float64(i)*1.5 {
			t.Fatalf("gauge %s = %f, want %f", name, val, float64(i)*1.5)
		}
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	models "github.com/puzakov/watchdog/internal/model"
)

type PostgresStorage struct {
	pool *pgxpool.Pool
}

var pgRetryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

func NewPostgresStorage(pool *pgxpool.Pool) *PostgresStorage {
	return &PostgresStorage{pool: pool}
}

func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	if s == nil || s.pool == nil {
		return 0, false
	}

	const q = `SELECT value FROM metrics WHERE id=$1 AND mtype=$2`
	var v float64
	for attempt := 0; ; attempt++ {
		err := s.pool.QueryRow(context.Background(), q, name, models.Gauge).Scan(&v)
		if err == nil {
			return v, true
		}
		if err == pgx.ErrNoRows {
			return 0, false
		}
		if isRetriablePGConnError(err) && attempt < len(pgRetryDelays) {
			time.Sleep(pgRetryDelays[attempt])
			continue
		}
		return 0, false
	}
}

func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	if s == nil || s.pool == nil {
		return 0, false
	}

	const q = `SELECT delta FROM metrics WHERE id=$1 AND mtype=$2`
	var v int64
	for attempt := 0; ; attempt++ {
		err := s.pool.QueryRow(context.Background(), q, name, models.Counter).Scan(&v)
		if err == nil {
			return v, true
		}
		if err == pgx.ErrNoRows {
			return 0, false
		}
		if isRetriablePGConnError(err) && attempt < len(pgRetryDelays) {
			time.Sleep(pgRetryDelays[attempt])
			continue
		}
		return 0, false
	}
}

func (s *PostgresStorage) UpdateGauge(name string, value float64) error {
	if s == nil || s.pool == nil {
		return nil
	}

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, NOW())
ON CONFLICT (id, mtype)
DO UPDATE SET value = EXCLUDED.value, delta = NULL, updated_at = NOW()`

	return execWithRetry(s.pool, q, name, models.Gauge, value)
}

func (s *PostgresStorage) UpdateCounter(name string, delta int64) error {
	if s == nil || s.pool == nil {
		return nil
	}

	const q = `
INSERT INTO metrics (id, mtype, delta, value, updated_at)
VALUES ($1, $2, $3, NULL, NOW())
ON CONFLICT (id, mtype)
DO UPDATE SET delta = metrics.delta + EXCLUDED.delta, value = NULL, updated_at = NOW()`

	return execWithRetry(s.pool, q, name, models.Counter, delta)
}

func (s *PostgresStorage) UpdateBatch(metrics []models.Metrics) error {
	if s == nil || s.pool == nil {
		return nil
	}
	if len(metrics) == 0 {
		return nil
	}

	// Deduplicate to avoid "ON CONFLICT DO UPDATE command cannot affect row a second time".
	// - gauge: last value wins
	// - counter: sum deltas
	gauges := make(map[string]float64)
	counters := make(map[string]int64)
	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if m.Value == nil {
				continue
			}
			gauges[m.ID] = *m.Value
		case models.Counter:
			if m.Delta == nil {
				continue
			}
			counters[m.ID] += *m.Delta
		default:
			// ignore unknown
		}
	}

	for attempt := 0; ; attempt++ {
		err := s.updateBatchOnce(gauges, counters)
		if err == nil {
			return nil
		}
		if isRetriablePGConnError(err) && attempt < len(pgRetryDelays) {
			time.Sleep(pgRetryDelays[attempt])
			continue
		}
		return err
	}
}

func (s *PostgresStorage) Snapshot() (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)
	if s == nil || s.pool == nil {
		return gauges, counters
	}

	const q = `SELECT id, mtype, delta, value FROM metrics`
	var rows pgx.Rows
	for attempt := 0; ; attempt++ {
		r, err := s.pool.Query(context.Background(), q)
		if err == nil {
			rows = r
			break
		}
		if isRetriablePGConnError(err) && attempt < len(pgRetryDelays) {
			time.Sleep(pgRetryDelays[attempt])
			continue
		}
		return gauges, counters
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id    string
			mtype string
			delta *int64
			value *float64
		)
		if err := rows.Scan(&id, &mtype, &delta, &value); err != nil {
			continue
		}
		switch mtype {
		case models.Gauge:
			if value == nil {
				continue
			}
			gauges[id] = *value
		case models.Counter:
			if delta == nil {
				continue
			}
			counters[id] = *delta
		}
	}

	return gauges, counters
}

func execWithRetry(pool *pgxpool.Pool, query string, args ...any) error {
	for attempt := 0; ; attempt++ {
		_, err := pool.Exec(context.Background(), query, args...)
		if err == nil {
			return nil
		}
		if isRetriablePGConnError(err) && attempt < len(pgRetryDelays) {
			time.Sleep(pgRetryDelays[attempt])
			continue
		}
		return err
	}
}

func isRetriablePGConnError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgerrcode.IsConnectionException(pgErr.Code)
	}
	return false
}

func (s *PostgresStorage) updateBatchOnce(
	gauges map[string]float64,
	counters map[string]int64,
) error {
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if len(gauges) > 0 {
		ids := make([]string, 0, len(gauges))
		vals := make([]float64, 0, len(gauges))
		for id, v := range gauges {
			ids = append(ids, id)
			vals = append(vals, v)
		}

		const qGauge = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
SELECT unnest($1::text[]), $3, unnest($2::float8[]), NULL, NOW()
ON CONFLICT (id, mtype)
DO UPDATE SET value = EXCLUDED.value, delta = NULL, updated_at = NOW()`

		if _, err := tx.Exec(context.Background(), qGauge, ids, vals, models.Gauge); err != nil {
			return fmt.Errorf("batch upsert gauges: %w", err)
		}
	}

	if len(counters) > 0 {
		ids := make([]string, 0, len(counters))
		deltas := make([]int64, 0, len(counters))
		for id, d := range counters {
			ids = append(ids, id)
			deltas = append(deltas, d)
		}

		const qCounter = `
INSERT INTO metrics (id, mtype, delta, value, updated_at)
SELECT unnest($1::text[]), $3, unnest($2::bigint[]), NULL, NOW()
ON CONFLICT (id, mtype)
DO UPDATE SET delta = metrics.delta + EXCLUDED.delta, value = NULL, updated_at = NOW()`

		if _, err := tx.Exec(context.Background(), qCounter, ids, deltas, models.Counter); err != nil {
			return fmt.Errorf("batch upsert counters: %w", err)
		}
	}

	return tx.Commit(context.Background())
}

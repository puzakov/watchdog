package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	models "github.com/puzakov/watchdog/internal/model"
)

type PostgresStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresStorage(pool *pgxpool.Pool) *PostgresStorage {
	return &PostgresStorage{pool: pool}
}

func (s *PostgresStorage) GetGauge(name string) (float64, bool) {
	if s == nil || s.pool == nil {
		return 0, false
	}

	const q = `SELECT value FROM metrics WHERE id=$1 AND mtype=$2`
	var v float64
	err := s.pool.QueryRow(context.Background(), q, name, models.Gauge).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false
		}
		return 0, false
	}
	return v, true
}

func (s *PostgresStorage) GetCounter(name string) (int64, bool) {
	if s == nil || s.pool == nil {
		return 0, false
	}

	const q = `SELECT delta FROM metrics WHERE id=$1 AND mtype=$2`
	var v int64
	err := s.pool.QueryRow(context.Background(), q, name, models.Counter).Scan(&v)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false
		}
		return 0, false
	}
	return v, true
}

func (s *PostgresStorage) UpdateGauge(name string, value float64) {
	if s == nil || s.pool == nil {
		return
	}

	const q = `
INSERT INTO metrics (id, mtype, value, delta, updated_at)
VALUES ($1, $2, $3, NULL, NOW())
ON CONFLICT (id, mtype)
DO UPDATE SET value = EXCLUDED.value, delta = NULL, updated_at = NOW()`

	_, _ = s.pool.Exec(context.Background(), q, name, models.Gauge, value)
}

func (s *PostgresStorage) UpdateCounter(name string, delta int64) {
	if s == nil || s.pool == nil {
		return
	}

	const q = `
INSERT INTO metrics (id, mtype, delta, value, updated_at)
VALUES ($1, $2, $3, NULL, NOW())
ON CONFLICT (id, mtype)
DO UPDATE SET delta = metrics.delta + EXCLUDED.delta, value = NULL, updated_at = NOW()`

	_, _ = s.pool.Exec(context.Background(), q, name, models.Counter, delta)
}

func (s *PostgresStorage) Snapshot() (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)
	if s == nil || s.pool == nil {
		return gauges, counters
	}

	const q = `SELECT id, mtype, delta, value FROM metrics`
	rows, err := s.pool.Query(context.Background(), q)
	if err != nil {
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

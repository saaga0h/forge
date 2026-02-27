package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gpu-compute-orchestrator/pkg/patterns"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PatternStore handles database operations for behavioral patterns
type PatternStore struct {
	db *sql.DB
}

// NewPatternStore creates a new pattern store
func NewPatternStore(db *sql.DB) *PatternStore {
	return &PatternStore{db: db}
}

// SavePattern stores a behavioral pattern in the database
func (s *PatternStore) SavePattern(ctx context.Context, jobID string, chainID int, stats patterns.ChainStatistics, pattern *patterns.PatternDescription, signature string) error {
	query := `
		INSERT INTO behavioral_patterns (
			job_id, chain_id, signature_hash,
			name, description, intent, confidence,
			anchor_count, time_range, location_stats, temporal_pattern, characteristics,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		ON CONFLICT (job_id, chain_id) DO UPDATE SET
			signature_hash = EXCLUDED.signature_hash,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			intent = EXCLUDED.intent,
			confidence = EXCLUDED.confidence,
			anchor_count = EXCLUDED.anchor_count,
			time_range = EXCLUDED.time_range,
			location_stats = EXCLUDED.location_stats,
			temporal_pattern = EXCLUDED.temporal_pattern,
			characteristics = EXCLUDED.characteristics,
			updated_at = NOW()
		RETURNING id
	`

	// Marshal JSONB fields
	timeRangeJSON, err := json.Marshal(stats.TimeRange)
	if err != nil {
		return fmt.Errorf("marshal time_range: %w", err)
	}

	locationStatsJSON, err := json.Marshal(stats.LocationStats)
	if err != nil {
		return fmt.Errorf("marshal location_stats: %w", err)
	}

	temporalPatternJSON, err := json.Marshal(stats.TemporalPattern)
	if err != nil {
		return fmt.Errorf("marshal temporal_pattern: %w", err)
	}

	characteristicsJSON, err := json.Marshal(pattern.Characteristics)
	if err != nil {
		return fmt.Errorf("marshal characteristics: %w", err)
	}

	var id int
	err = s.db.QueryRowContext(ctx, query,
		jobID, chainID, signature,
		pattern.Name, pattern.Description, pattern.Intent, pattern.Confidence,
		stats.AnchorCount, timeRangeJSON, locationStatsJSON, temporalPatternJSON, characteristicsJSON,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("insert pattern: %w", err)
	}

	return nil
}

// GetPatternByJobAndChain retrieves a pattern by job ID and chain ID
func (s *PatternStore) GetPatternByJobAndChain(ctx context.Context, jobID string, chainID int) (*patterns.BehavioralPattern, error) {
	query := `
		SELECT id, job_id, chain_id, signature_hash,
		       name, description, intent, confidence,
		       anchor_count, time_range, location_stats, temporal_pattern, characteristics,
		       created_at, updated_at
		FROM behavioral_patterns
		WHERE job_id = $1 AND chain_id = $2
	`

	var bp patterns.BehavioralPattern
	var timeRangeJSON, locationStatsJSON, temporalPatternJSON, characteristicsJSON []byte

	err := s.db.QueryRowContext(ctx, query, jobID, chainID).Scan(
		&bp.ID, &bp.JobID, &bp.ChainID, &bp.SignatureHash,
		&bp.Name, &bp.Description, &bp.Intent, &bp.Confidence,
		&bp.AnchorCount, &timeRangeJSON, &locationStatsJSON, &temporalPatternJSON, &characteristicsJSON,
		&bp.CreatedAt, &bp.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query pattern: %w", err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(timeRangeJSON, &bp.TimeRange); err != nil {
		return nil, fmt.Errorf("unmarshal time_range: %w", err)
	}
	if err := json.Unmarshal(locationStatsJSON, &bp.LocationStats); err != nil {
		return nil, fmt.Errorf("unmarshal location_stats: %w", err)
	}
	if err := json.Unmarshal(temporalPatternJSON, &bp.TemporalPattern); err != nil {
		return nil, fmt.Errorf("unmarshal temporal_pattern: %w", err)
	}
	if err := json.Unmarshal(characteristicsJSON, &bp.Characteristics); err != nil {
		return nil, fmt.Errorf("unmarshal characteristics: %w", err)
	}

	return &bp, nil
}

// GetPatternsByJob retrieves all patterns for a job
func (s *PatternStore) GetPatternsByJob(ctx context.Context, jobID string) ([]patterns.BehavioralPattern, error) {
	query := `
		SELECT id, job_id, chain_id, signature_hash,
		       name, description, intent, confidence,
		       anchor_count, time_range, location_stats, temporal_pattern, characteristics,
		       created_at, updated_at
		FROM behavioral_patterns
		WHERE job_id = $1
		ORDER BY chain_id
	`

	rows, err := s.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("query patterns: %w", err)
	}
	defer rows.Close()

	var result []patterns.BehavioralPattern
	for rows.Next() {
		var bp patterns.BehavioralPattern
		var timeRangeJSON, locationStatsJSON, temporalPatternJSON, characteristicsJSON []byte

		err := rows.Scan(
			&bp.ID, &bp.JobID, &bp.ChainID, &bp.SignatureHash,
			&bp.Name, &bp.Description, &bp.Intent, &bp.Confidence,
			&bp.AnchorCount, &timeRangeJSON, &locationStatsJSON, &temporalPatternJSON, &characteristicsJSON,
			&bp.CreatedAt, &bp.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan pattern: %w", err)
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(timeRangeJSON, &bp.TimeRange); err != nil {
			return nil, fmt.Errorf("unmarshal time_range: %w", err)
		}
		if err := json.Unmarshal(locationStatsJSON, &bp.LocationStats); err != nil {
			return nil, fmt.Errorf("unmarshal location_stats: %w", err)
		}
		if err := json.Unmarshal(temporalPatternJSON, &bp.TemporalPattern); err != nil {
			return nil, fmt.Errorf("unmarshal temporal_pattern: %w", err)
		}
		if err := json.Unmarshal(characteristicsJSON, &bp.Characteristics); err != nil {
			return nil, fmt.Errorf("unmarshal characteristics: %w", err)
		}

		result = append(result, bp)
	}

	return result, rows.Err()
}

// CacheDescription stores a pattern description in the persistent cache
func (s *PatternStore) CacheDescription(ctx context.Context, signature string, pattern *patterns.PatternDescription) error {
	query := `
		INSERT INTO pattern_descriptions (signature_hash, pattern_description, first_seen, last_used, use_count)
		VALUES ($1, $2, NOW(), NOW(), 1)
		ON CONFLICT (signature_hash) DO UPDATE SET
			last_used = NOW(),
			use_count = pattern_descriptions.use_count + 1
	`

	patternJSON, err := json.Marshal(pattern)
	if err != nil {
		return fmt.Errorf("marshal pattern: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query, signature, patternJSON)
	if err != nil {
		return fmt.Errorf("cache description: %w", err)
	}

	return nil
}

// GetCachedDescription retrieves a cached pattern description
func (s *PatternStore) GetCachedDescription(ctx context.Context, signature string) (*patterns.PatternDescription, error) {
	query := `
		SELECT pattern_description
		FROM pattern_descriptions
		WHERE signature_hash = $1
	`

	var patternJSON []byte
	err := s.db.QueryRowContext(ctx, query, signature).Scan(&patternJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query cached description: %w", err)
	}

	var pattern patterns.PatternDescription
	if err := json.Unmarshal(patternJSON, &pattern); err != nil {
		return nil, fmt.Errorf("unmarshal pattern: %w", err)
	}

	return &pattern, nil
}

// GetRecentPatterns retrieves recent patterns (last N)
func (s *PatternStore) GetRecentPatterns(ctx context.Context, limit int) ([]patterns.BehavioralPattern, error) {
	query := `
		SELECT id, job_id, chain_id, signature_hash,
		       name, description, intent, confidence,
		       anchor_count, time_range, location_stats, temporal_pattern, characteristics,
		       created_at, updated_at
		FROM behavioral_patterns
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent patterns: %w", err)
	}
	defer rows.Close()

	var result []patterns.BehavioralPattern
	for rows.Next() {
		var bp patterns.BehavioralPattern
		var timeRangeJSON, locationStatsJSON, temporalPatternJSON, characteristicsJSON []byte

		err := rows.Scan(
			&bp.ID, &bp.JobID, &bp.ChainID, &bp.SignatureHash,
			&bp.Name, &bp.Description, &bp.Intent, &bp.Confidence,
			&bp.AnchorCount, &timeRangeJSON, &locationStatsJSON, &temporalPatternJSON, &characteristicsJSON,
			&bp.CreatedAt, &bp.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan pattern: %w", err)
		}

		// Unmarshal JSONB fields
		json.Unmarshal(timeRangeJSON, &bp.TimeRange)
		json.Unmarshal(locationStatsJSON, &bp.LocationStats)
		json.Unmarshal(temporalPatternJSON, &bp.TemporalPattern)
		json.Unmarshal(characteristicsJSON, &bp.Characteristics)

		result = append(result, bp)
	}

	return result, rows.Err()
}

// GetCacheStats returns cache statistics
func (s *PatternStore) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_entries,
			SUM(use_count) as total_hits,
			AVG(use_count) as avg_hits_per_entry,
			MAX(use_count) as max_hits,
			MIN(first_seen) as oldest_entry,
			MAX(last_used) as most_recent_access
		FROM pattern_descriptions
	`

	var totalEntries, totalHits, maxHits int
	var avgHits float64
	var oldestEntry, mostRecent time.Time

	err := s.db.QueryRowContext(ctx, query).Scan(
		&totalEntries, &totalHits, &avgHits, &maxHits, &oldestEntry, &mostRecent,
	)
	if err != nil {
		return nil, fmt.Errorf("query cache stats: %w", err)
	}

	return map[string]interface{}{
		"total_entries":       totalEntries,
		"total_hits":          totalHits,
		"avg_hits_per_entry":  avgHits,
		"max_hits":            maxHits,
		"oldest_entry":        oldestEntry,
		"most_recent_access":  mostRecent,
	}, nil
}

// ConnectDB creates a database connection
func ConnectDB(host string, port int, user, password, dbname string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)

	return db, nil
}

-- Migration: Create pattern storage tables
-- Purpose: Store behavioral patterns from semantic diffusion + LLM transcoding
-- Created: 2025-11-14

-- ============================================================================
-- Behavioral Patterns (Final transcoded patterns)
-- ============================================================================

CREATE TABLE IF NOT EXISTS behavioral_patterns (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(255) NOT NULL,
    chain_id INT NOT NULL,
    signature_hash VARCHAR(64) NOT NULL,

    -- Pattern metadata (from LLM)
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    intent TEXT,
    confidence VARCHAR(20),  -- 'high', 'medium', 'low'

    -- Statistics (for debugging/analysis)
    anchor_count INT NOT NULL,
    time_range JSONB,           -- TimeRangeStats
    location_stats JSONB,       -- map[string]int
    temporal_pattern JSONB,     -- TemporalStats
    characteristics JSONB,      -- PatternCharacteristics

    -- Tracking
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Indexes
    CONSTRAINT behavioral_patterns_job_chain UNIQUE (job_id, chain_id)
);

CREATE INDEX IF NOT EXISTS idx_patterns_signature ON behavioral_patterns(signature_hash);
CREATE INDEX IF NOT EXISTS idx_patterns_job ON behavioral_patterns(job_id);
CREATE INDEX IF NOT EXISTS idx_patterns_created ON behavioral_patterns(created_at);
CREATE INDEX IF NOT EXISTS idx_patterns_name ON behavioral_patterns(name);

-- ============================================================================
-- Pattern Descriptions Cache (For LLM response caching)
-- ============================================================================

CREATE TABLE IF NOT EXISTS pattern_descriptions (
    signature_hash VARCHAR(64) PRIMARY KEY,
    pattern_description JSONB NOT NULL,  -- Full PatternDescription object
    first_seen TIMESTAMP DEFAULT NOW(),
    last_used TIMESTAMP DEFAULT NOW(),
    use_count INT DEFAULT 1,

    -- Metadata for cache management
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_descriptions_last_used ON pattern_descriptions(last_used);
CREATE INDEX IF NOT EXISTS idx_descriptions_use_count ON pattern_descriptions(use_count);

-- ============================================================================
-- Pattern History (Phase 3 - Pattern evolution tracking)
-- ============================================================================

CREATE TABLE IF NOT EXISTS pattern_history (
    id SERIAL PRIMARY KEY,
    signature_hash VARCHAR(64) NOT NULL,
    observed_at TIMESTAMP NOT NULL,

    -- Snapshot of pattern at this time
    anchor_count INT,
    time_range JSONB,
    location_stats JSONB,
    temporal_pattern JSONB,

    -- Pattern drift detection
    similarity_to_previous FLOAT,  -- 0-1 score vs previous occurrence

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_history_signature_time ON pattern_history(signature_hash, observed_at);
CREATE INDEX IF NOT EXISTS idx_history_observed ON pattern_history(observed_at);

-- ============================================================================
-- Functions and Triggers
-- ============================================================================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_behavioral_patterns_updated_at BEFORE UPDATE
    ON behavioral_patterns FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Update pattern_descriptions last_used and use_count on access
CREATE OR REPLACE FUNCTION update_pattern_cache_stats()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_used = NOW();
    NEW.use_count = OLD.use_count + 1;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_pattern_descriptions_stats BEFORE UPDATE
    ON pattern_descriptions FOR EACH ROW
    EXECUTE FUNCTION update_pattern_cache_stats();

-- ============================================================================
-- Helper Views
-- ============================================================================

-- View: Recent patterns with useful metadata
CREATE OR REPLACE VIEW recent_patterns AS
SELECT
    bp.id,
    bp.job_id,
    bp.chain_id,
    bp.name,
    bp.confidence,
    bp.anchor_count,
    bp.created_at,
    bp.characteristics->>'frequency' as frequency,
    bp.characteristics->>'time_of_day' as time_of_day,
    ARRAY(
        SELECT jsonb_array_elements_text(bp.characteristics->'primary_locations')
    ) as primary_locations
FROM behavioral_patterns bp
ORDER BY bp.created_at DESC
LIMIT 100;

-- View: Cache performance statistics
CREATE OR REPLACE VIEW cache_stats AS
SELECT
    COUNT(*) as total_entries,
    SUM(use_count) as total_hits,
    AVG(use_count) as avg_hits_per_entry,
    MAX(use_count) as max_hits,
    MIN(first_seen) as oldest_entry,
    MAX(last_used) as most_recent_access
FROM pattern_descriptions;

-- ============================================================================
-- Sample Queries (Comments for documentation)
-- ============================================================================

-- Find all morning routines:
-- SELECT * FROM behavioral_patterns
-- WHERE characteristics->>'time_of_day' = 'morning'
-- ORDER BY created_at DESC;

-- Find patterns using specific location:
-- SELECT * FROM behavioral_patterns
-- WHERE location_stats ? 'kitchen'  -- JSONB contains key
-- ORDER BY (location_stats->>'kitchen')::int DESC;

-- Check cache hit rate:
-- SELECT * FROM cache_stats;

-- Find pattern evolution over time:
-- SELECT signature_hash, observed_at, anchor_count,
--        similarity_to_previous
-- FROM pattern_history
-- WHERE signature_hash = 'abc123...'
-- ORDER BY observed_at;

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	pb "github.com/MateuszBrankiewicz/cloudguardian/server/proto"
)

var schema = `
CREATE TABLE IF NOT EXISTS resources (
    resource_id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    cost DOUBLE PRECISION NOT NULL,
    tags JSONB,
    is_public BOOLEAN NOT NULL,
    dependencies JSONB,
    ai_recommendation TEXT,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pii_findings (
    id SERIAL PRIMARY KEY,
    resource_id TEXT NOT NULL,
    pii_type TEXT NOT NULL,
    confidence FLOAT NOT NULL,
    occurrence_count INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);`

type DB struct {
	*sqlx.DB
}

func InitDB(dataSourceName string) (*DB, error) {
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Simple migrations: add columns if they don't exist
	_, _ = db.Exec("ALTER TABLE resources ADD COLUMN IF NOT EXISTS ai_recommendation TEXT")
	_, _ = db.Exec("ALTER TABLE resources ADD COLUMN IF NOT EXISTS dependencies JSONB")

	log.Println("✅ Database initialized")
	return &DB{db}, nil
}

func (db *DB) SaveResource(r *pb.InfrastructureResource) error {
	tagsJSON, err := json.Marshal(r.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	depsJSON, err := json.Marshal(r.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	query := `
		INSERT INTO resources (resource_id, provider, resource_type, cost, tags, is_public, dependencies, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (resource_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			resource_type = EXCLUDED.resource_type,
			cost = EXCLUDED.cost,
			tags = EXCLUDED.tags,
			is_public = EXCLUDED.is_public,
			dependencies = EXCLUDED.dependencies,
			updated_at = EXCLUDED.updated_at
	`

	_, err = db.Exec(query,
		r.ResourceId,
		r.Provider,
		r.Type,
		r.EstimatedCost,
		tagsJSON,
		r.IsPublic,
		depsJSON,
		time.Now(),
	)

	return err
}

func (db *DB) SavePIIFinding(f *pb.PIIResult) error {
	query := `
		INSERT INTO pii_findings (resource_id, pii_type, confidence, occurrence_count)
		VALUES ($1, $2, $3, $4)
	`
	_, err := db.Exec(query, f.ResourceId, f.DataType, f.Confidence, f.OccurrenceCount)
	return err
}

type PIIFinding struct {
	ResourceId      string  `db:"resource_id" json:"resource_id"`
	PiiType         string  `db:"pii_type" json:"pii_type"`
	OccurrenceCount int32   `db:"occurrence_count" json:"occurrence_count"`
	Confidence      float32 `db:"confidence" json:"confidence"`
}

type ResourceRow struct {
	ResourceId       string           `db:"resource_id" json:"resource_id"`
	Provider         string           `db:"provider" json:"provider"`
	ResourceType     string           `db:"resource_type" json:"resource_type"`
	Cost             float64          `db:"cost" json:"cost"`
	Tags             *json.RawMessage `db:"tags" json:"tags"`
	IsPublic         bool             `db:"is_public" json:"is_public"`
	Dependencies     *json.RawMessage `db:"dependencies" json:"dependencies"`
	AiRecommendation sql.NullString   `db:"ai_recommendation" json:"-"`
	AiRecString      string           `json:"ai_recommendation"`
	UpdatedAt        time.Time        `db:"updated_at" json:"updated_at"`
	HasPII           bool             `db:"has_pii" json:"has_pii"`
}

func (db *DB) GetAllResources() ([]ResourceRow, error) {
	var rows []ResourceRow
	query := `
		SELECT 
			resource_id, provider, resource_type, cost, tags, is_public, dependencies, ai_recommendation, updated_at,
			EXISTS(SELECT 1 FROM pii_findings f WHERE f.resource_id = resources.resource_id) as has_pii
		FROM resources
	`
	err := db.Select(&rows, query)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].AiRecommendation.Valid {
			rows[i].AiRecString = rows[i].AiRecommendation.String
		}
	}
	return rows, nil
}

func (db *DB) GetAllPIIFindings() ([]PIIFinding, error) {
	var findings []PIIFinding
	err := db.Select(&findings, "SELECT resource_id, pii_type, occurrence_count, confidence FROM pii_findings")
	return findings, err
}

type Summary struct {
	TotalCost         float64 `json:"total_cost"`
	TotalPII          int     `json:"total_pii"`
	HighRiskResources int     `json:"high_risk_resources"`
}

func (db *DB) GetSummary() (Summary, error) {
	var s Summary
	db.Get(&s.TotalCost, "SELECT COALESCE(SUM(cost), 0) FROM resources")
	db.Get(&s.TotalPII, "SELECT COALESCE(SUM(occurrence_count), 0) FROM pii_findings")
	db.Get(&s.HighRiskResources, "SELECT COUNT(DISTINCT resource_id) FROM resources WHERE is_public = true AND resource_id IN (SELECT resource_id FROM pii_findings)")
	return s, nil
}

func (db *DB) GetResourceWithFindings(resourceID string) (*pb.InfrastructureResource, []PIIFinding, error) {
	var res pb.InfrastructureResource
	var tagsRaw, depsRaw []byte
	var aiRec sql.NullString

	err := db.QueryRow(`SELECT resource_id, provider, resource_type, cost, tags, is_public, dependencies, ai_recommendation 
		FROM resources WHERE resource_id = $1`, resourceID).Scan(
		&res.ResourceId, &res.Provider, &res.Type, &res.EstimatedCost, &tagsRaw, &res.IsPublic, &depsRaw, &aiRec)
	
	if err != nil {
		return nil, nil, err
	}

	if tagsRaw != nil {
		json.Unmarshal(tagsRaw, &res.Tags)
	}
	if depsRaw != nil {
		json.Unmarshal(depsRaw, &res.Dependencies)
	}

	var findings []PIIFinding
	err = db.Select(&findings, "SELECT resource_id, pii_type, occurrence_count, confidence FROM pii_findings WHERE resource_id = $1", resourceID)
	
	return &res, findings, err
}

func (db *DB) UpdateAIRecommendation(resourceID, recommendation string) error {
	_, err := db.Exec("UPDATE resources SET ai_recommendation = $1, updated_at = $2 WHERE resource_id = $3", 
		recommendation, time.Now(), resourceID)
	return err
}

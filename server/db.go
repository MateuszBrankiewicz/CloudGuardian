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
	PiiType         string  `db:"pii_type"`
	OccurrenceCount int32   `db:"occurrence_count"`
	Confidence      float32 `db:"confidence"`
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
	err = db.Select(&findings, "SELECT pii_type, occurrence_count, confidence FROM pii_findings WHERE resource_id = $1", resourceID)
	
	return &res, findings, err
}

func (db *DB) UpdateAIRecommendation(resourceID, recommendation string) error {
	_, err := db.Exec("UPDATE resources SET ai_recommendation = $1, updated_at = $2 WHERE resource_id = $3", 
		recommendation, time.Now(), resourceID)
	return err
}

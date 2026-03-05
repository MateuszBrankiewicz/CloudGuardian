package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/MateuszBrankiewicz/cloudguardian/server/proto"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	ctx := context.Background()

	dbName := "cloudguardian"
	dbUser := "admin"
	dbPassword := "password"

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10 * time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	db, err := InitDB(connStr)
	if err != nil {
		t.Fatalf("failed to init db: %s", err)
	}

	return db, func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}
}

func TestReportResourceValidation(t *testing.T) {
	// Initialize with nil db to test validation only
	s := &server{db: nil}

	// Case 1: Negative cost
	_, err := s.ReportResource(context.Background(), &pb.InfrastructureResource{
		ResourceId:    "test-1",
		EstimatedCost: -10.0,
	})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// Case 2: Missing resource_id
	_, err = s.ReportResource(context.Background(), &pb.InfrastructureResource{
		EstimatedCost: 10.0,
	})
	assert.Error(t, err)
	st, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestReportResourcePersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()

	s := &server{db: db}

	req := &pb.InfrastructureResource{
		ResourceId:    "s3-bucket-1",
		Provider:      "aws",
		Type:          "s3",
		EstimatedCost: 50.0,
		Tags:          map[string]string{"env": "prod"},
		IsPublic:      true,
		Dependencies:  []string{"aws_vpc.main"},
	}

	// First save (Insert)
	resp, err := s.ReportResource(context.Background(), req)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify in DB
	var count int
	err = db.Get(&count, "SELECT count(*) FROM resources WHERE resource_id = $1", req.ResourceId)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	var cost float64
	err = db.Get(&cost, "SELECT cost FROM resources WHERE resource_id = $1", req.ResourceId)
	assert.NoError(t, err)
	assert.Equal(t, 50.0, cost)

	var depsJSON []byte
	err = db.Get(&depsJSON, "SELECT dependencies FROM resources WHERE resource_id = $1", req.ResourceId)
	assert.NoError(t, err)
	assert.Contains(t, string(depsJSON), "aws_vpc.main")

	// Update (Upsert)
	req.EstimatedCost = 75.0
	_, err = s.ReportResource(context.Background(), req)
	assert.NoError(t, err)

	err = db.Get(&cost, "SELECT cost FROM resources WHERE resource_id = $1", req.ResourceId)
	assert.NoError(t, err)
	assert.Equal(t, 75.0, cost)
}

func TestReportPIIFindingPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()

	s := &server{db: db}

	req := &pb.PIIResult{
		ResourceId:      "data.csv",
		DataType:        "pesel",
		Confidence:      1.0,
		OccurrenceCount: 5,
	}

	resp, err := s.ReportPIIFinding(context.Background(), req)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify in DB
	var count int
	err = db.Get(&count, "SELECT count(*) FROM pii_findings WHERE resource_id = $1 AND pii_type = $2", req.ResourceId, req.DataType)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	var occ int
	err = db.Get(&occ, "SELECT occurrence_count FROM pii_findings WHERE resource_id = $1", req.ResourceId)
	assert.NoError(t, err)
	assert.Equal(t, 5, occ)
}

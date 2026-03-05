package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/MateuszBrankiewicz/cloudguardian/server/ai"
	pb "github.com/MateuszBrankiewicz/cloudguardian/server/proto"
)

type server struct {
	pb.UnimplementedScannerServiceServer
	db      *DB
	advisor *ai.Advisor
}

func (s *server) ReportResource(ctx context.Context, req *pb.InfrastructureResource) (*pb.ScanResponse, error) {
	if req.ResourceId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "resource_id is required")
	}
	if req.EstimatedCost < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "estimated_cost cannot be negative: %.2f", req.EstimatedCost)
	}

	log.Printf("📥 Otrzymano zasób: [%s] %s - Koszt: $%.2f", req.Provider, req.ResourceId, req.EstimatedCost)

	if err := s.db.SaveResource(req); err != nil {
		log.Printf("❌ Błąd zapisu do bazy: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to save resource: %v", err)
	}

	return &pb.ScanResponse{
		Success: true,
		Message: "Zasób zapisany poprawnie w bazie danych!",
	}, nil
}

func (s *server) ReportPIIFinding(ctx context.Context, req *pb.PIIResult) (*pb.ScanResponse, error) {
	log.Printf("📥 Otrzymano PII: [%s] %s - Pewność: %.2f (Ilość: %d)", req.ResourceId, req.DataType, req.Confidence, req.OccurrenceCount)
	
	if err := s.db.SavePIIFinding(req); err != nil {
		log.Printf("❌ Błąd zapisu PII do bazy: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to save PII finding: %v", err)
	}

	return &pb.ScanResponse{
		Success: true,
		Message: "PII zarejestrowany poprawnie w bazie danych!",
	}, nil
}

func (s *server) handleFix(w http.ResponseWriter, r *http.Request) {
	// Simple path parsing for /api/resources/:id/fix
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	resourceID := parts[3]

	log.Printf("🤖 Generowanie poprawki dla: %s", resourceID)

	res, findings, err := s.db.GetResourceWithFindings(resourceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Resource not found: %v", err), http.StatusNotFound)
		return
	}

	// Map findings to ai.PIIFinding
	aiFindings := make([]ai.PIIFinding, len(findings))
	for i, f := range findings {
		aiFindings[i] = ai.PIIFinding{
			PiiType:         f.PiiType,
			OccurrenceCount: f.OccurrenceCount,
		}
	}

	fix, err := s.advisor.GenerateRemediation(res, aiFindings)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate fix: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, fix)
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://admin:password@localhost:5432/cloudguardian?sslmode=disable"
	}

	db, err := InitDB(dsn)
	if err != nil {
		log.Fatalf("❌ Błąd inicjalizacji bazy: %v", err)
	}

	ollamaURL := os.Getenv("OLLAMA_URL")
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "llama3:latest"
	}
	ollamaClient := ai.NewOllamaClient(ollamaURL)
	advisor := ai.NewAdvisor(ollamaClient, ollamaModel)

	srv := &server{
		db:      db,
		advisor: advisor,
	}

	// [AC 2] HTTP API for remediation
	http.HandleFunc("/api/resources/", srv.handleFix)
	go func() {
		log.Println("🌐 API HTTP nasłuchuje na :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("❌ Błąd serwera HTTP: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Błąd startu nasłuchiwania: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterScannerServiceServer(s, srv)

	log.Println("🚀 Serwer gRPC CloudGuardian nasłuchuje na :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("❌ Błąd serwera gRPC: %v", err)
	}
}

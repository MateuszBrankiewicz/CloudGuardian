package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/MateuszBrankiewicz/cloudguardian/server/proto"
)

type server struct {
	pb.UnimplementedScannerServiceServer
	db *DB
}

func (s *server) ReportResource(ctx context.Context, req *pb.InfrastructureResource) (*pb.ScanResponse, error) {
	// [AC 1] Walidacja danych
	if req.ResourceId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "resource_id is required")
	}
	if req.EstimatedCost < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "estimated_cost cannot be negative: %.2f", req.EstimatedCost)
	}

	log.Printf("📥 Otrzymano zasób: [%s] %s - Koszt: $%.2f", req.Provider, req.ResourceId, req.EstimatedCost)

	// [AC 3] Trwały zapis (Upsert)
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
	log.Printf("📥 Otrzymano PII: [%s] %s - Pewność: %.2f", req.ResourceId, req.DataType, req.Confidence)
	return &pb.ScanResponse{
		Success: true,
		Message: "PII zarejestrowany poprawnie!",
	}, nil
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://admin:password@localhost:5432/cloudguardian?sslmode=disable"
	}

	// [AC 2] Integracja z SQL
	db, err := InitDB(dsn)
	if err != nil {
		log.Fatalf("❌ Błąd inicjalizacji bazy: %v", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Błąd startu nasłuchiwania: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterScannerServiceServer(s, &server{db: db})

	log.Println("🚀 Serwer Go CloudGuardian nasłuchuje na :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("❌ Błąd serwera: %v", err)
	}
}

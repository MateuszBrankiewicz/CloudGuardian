package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	// TUTAJ: Zmień "github.com/twoja-nazwa/cloudGuardian/server/proto"
	// na ścieżkę zgodną z Twoim plikiem go.mod + folder, do którego wrzuciłeś pliki .pb.go
	pb "github.com/MateuszBrankiewicz/cloudguardian/server/proto"
)

// Serwer musi zawierać UnimplementedScannerServiceServer
type server struct {
	pb.UnimplementedScannerServiceServer
}

// Musisz użyć pb. przy typach argumentów i zwracanych danych
func (s *server) ReportResource(ctx context.Context, req *pb.InfrastructureResource) (*pb.ScanResponse, error) {
	log.Printf("📥 Otrzymano zasób: [%s] %s - Koszt: $%.2f", req.Provider, req.EstimatedCost)

	return &pb.ScanResponse{
		Succes:  true,
		Message: "Zasób zarejestrowany poprawnie w Go!",
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Błąd startu nasłuchiwania: %v", err)
	}

	s := grpc.NewServer()

	// TUTAJ: Odkomentowane i poprawione - rejestrujemy naszą usługę
	pb.RegisterScannerServiceServer(s, &server{})

	log.Println("🚀 Serwer Go CloudGuardian nasłuchuje na :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("❌ Błąd serwera: %v", err)
	}
}

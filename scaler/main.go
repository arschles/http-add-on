package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/kedacore/http-add-on/pkg/http"
	externalscaler "github.com/kedacore/http-add-on/scaler/gen"
	"google.golang.org/grpc"
)

func main() {
	portStr := os.Getenv("PORT")
	q := http.NewMemoryQueue()
	log.Fatal(startGrpcServer(portStr, q))
}

func startGrpcServer(port string, q http.QueueCountReader) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, newImpl(q))
	return grpcServer.Serve(lis)
}

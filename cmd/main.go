package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/benrod407/explore-service/explore_service_proto"
	service "github.com/benrod407/explore-service/internal"
	"google.golang.org/grpc"
)

func main() {

	dbName := getEnv("MYSQL_DATABASE", "myapp_db")
	dbHost := getEnv("MYSQL_HOST", "127.0.0.1")
	dbPort := getEnv("MYSQL_PORT", "3306")
	dbUser := getEnv("MYSQL_USER", "root")
	dbPassword := getEnv("MYSQL_PASSWORD", "rootsecret")

	ctx := context.Background()

	// connect to DB instance
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbHost, dbPort, dbName)
	dbInstance, err := service.NewDB(ctx, dataSourceName)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer dbInstance.Close()

	// initialice explore-service server
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Print("listening on port 9001")

	grpcServer := grpc.NewServer()

	// Create business logic layer
	business := service.NewExploreBusiness(dbInstance)

	// Create gRPC handler with business logic dependency
	pb.RegisterExploreServiceServer(grpcServer, &service.ExploreService{
		Business: business,
	})

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

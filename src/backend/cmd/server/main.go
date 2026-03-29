package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/mrralexandrov/debt-bot/backend/gen/debt/v1"
	"github.com/mrralexandrov/debt-bot/backend/internal/grpchandler"
	"github.com/mrralexandrov/debt-bot/backend/internal/repository/postgres"
	"github.com/mrralexandrov/debt-bot/backend/internal/schema"
	"github.com/mrralexandrov/debt-bot/backend/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dsn := mustEnv("DATABASE_URL")
	port := envOr("GRPC_PORT", "50051")

	ctx := context.Background()

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if err := schema.Apply(ctx, db); err != nil {
		log.Fatalf("apply schema: %v", err)
	}

	userRepo := postgres.NewUserRepository(db)
	dealRepo := postgres.NewDealRepository(db)
	purchaseRepo := postgres.NewPurchaseRepository(db)

	userSvc := service.NewUserService(userRepo)
	dealSvc := service.NewDealService(dealRepo, purchaseRepo)
	debtSvc := service.NewDebtService(dealRepo, purchaseRepo)

	handler := grpchandler.New(userSvc, dealSvc, debtSvc)

	srv := grpc.NewServer()
	pb.RegisterDebtServiceServer(srv, handler)
	reflection.Register(srv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	log.Printf("gRPC server listening on :%s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

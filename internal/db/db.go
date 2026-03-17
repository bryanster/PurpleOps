package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bryanster/purpleops/internal/config"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	// DB is the package-level MongoDB database instance.
	DB          *mongo.Database
	mongoClient *mongo.Client
)

// InitDB connects to MongoDB using the provided config and sets the package-level DB var.
func InitDB(cfg *config.Config) {
	uri := fmt.Sprintf("mongodb://%s:%d", cfg.MongoHost, cfg.MongoPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	mongoClient = client
	DB = client.Database(cfg.MongoDB)
	log.Println("Connected to MongoDB")
}

// Col returns a MongoDB collection by name from the package-level database.
func Col(name string) *mongo.Collection {
	return DB.Collection(name)
}

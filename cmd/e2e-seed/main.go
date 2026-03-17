package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	_ = godotenv.Load()

	mongoHost := envOrDefault("MONGO_HOST", "localhost")
	mongoPort := envOrDefault("MONGO_PORT", "27017")
	mongoDB := envOrDefault("MONGO_DB", "purpleops_e2e")

	uri := fmt.Sprintf("mongodb://%s:%s", mongoHost, mongoPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	db := client.Database(mongoDB)

	// Drop the test database for a clean slate.
	if err := db.Drop(ctx); err != nil {
		log.Fatalf("Failed to drop database: %v", err)
	}
	fmt.Printf("Dropped database %s\n", mongoDB)

	// Seed roles.
	roleColl := db.Collection("role")
	adminRoleID := bson.NewObjectID()
	redRoleID := bson.NewObjectID()
	blueRoleID := bson.NewObjectID()
	spectatorRoleID := bson.NewObjectID()

	roles := []interface{}{
		bson.D{{Key: "_id", Value: adminRoleID}, {Key: "name", Value: "Admin"}},
		bson.D{{Key: "_id", Value: redRoleID}, {Key: "name", Value: "Red"}},
		bson.D{{Key: "_id", Value: blueRoleID}, {Key: "name", Value: "Blue"}},
		bson.D{{Key: "_id", Value: spectatorRoleID}, {Key: "name", Value: "Spectator"}},
	}

	if _, err := roleColl.InsertMany(ctx, roles); err != nil {
		log.Fatalf("Failed to insert roles: %v", err)
	}
	fmt.Println("Inserted roles: Admin, Red, Blue, Spectator")

	// Seed admin user with known password.
	password := "testpassword123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	userColl := db.Collection("user")
	_, err = userColl.InsertOne(ctx, bson.D{
		{Key: "email", Value: "admin@purpleops.com"},
		{Key: "username", Value: "admin"},
		{Key: "password", Value: string(hashedPassword)},
		{Key: "roles", Value: bson.A{adminRoleID}},
		{Key: "assessments", Value: bson.A{}},
		{Key: "active", Value: true},
		{Key: "initpwd", Value: false},
	})
	if err != nil {
		log.Fatalf("Failed to insert admin user: %v", err)
	}
	fmt.Printf("Admin user created: admin@purpleops.com / %s\n", password)

	// Seed a minimal tactic for testcase creation.
	tacticColl := db.Collection("tactic")
	tactics := []interface{}{
		bson.D{{Key: "mitreid", Value: "TA0001"}, {Key: "name", Value: "Initial Access"}},
		bson.D{{Key: "mitreid", Value: "TA0002"}, {Key: "name", Value: "Execution"}},
		bson.D{{Key: "mitreid", Value: "TA0003"}, {Key: "name", Value: "Persistence"}},
	}
	if _, err := tacticColl.InsertMany(ctx, tactics); err != nil {
		log.Fatalf("Failed to insert tactics: %v", err)
	}
	fmt.Println("Inserted 3 tactics")

	// Seed a minimal technique for testcase creation.
	techColl := db.Collection("technique")
	techniques := []interface{}{
		bson.D{
			{Key: "mitreid", Value: "T1059"},
			{Key: "name", Value: "Command and Scripting Interpreter"},
			{Key: "description", Value: "Test technique"},
			{Key: "detection", Value: "Monitor command-line"},
			{Key: "tactics", Value: bson.A{"execution"}},
		},
		bson.D{
			{Key: "mitreid", Value: "T1566"},
			{Key: "name", Value: "Phishing"},
			{Key: "description", Value: "Test phishing technique"},
			{Key: "detection", Value: "Monitor emails"},
			{Key: "tactics", Value: bson.A{"initial-access"}},
		},
	}
	if _, err := techColl.InsertMany(ctx, techniques); err != nil {
		log.Fatalf("Failed to insert techniques: %v", err)
	}
	fmt.Println("Inserted 2 techniques")

	fmt.Println("\nE2E seed complete. Ready for testing.")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

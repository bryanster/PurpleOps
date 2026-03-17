package models

import (
	"context"

	"github.com/bryanster/purpleops/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// FindAssessment retrieves an assessment by its hex ID.
func FindAssessment(ctx context.Context, id string) (*Assessment, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var a Assessment
	err = db.Col("assessment").FindOne(ctx, bson.M{"_id": oid}).Decode(&a)
	return &a, err
}

// FindTestCase retrieves a testcase by its hex ID.
func FindTestCase(ctx context.Context, id string) (*TestCase, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var tc TestCase
	err = db.Col("test_case").FindOne(ctx, bson.M{"_id": oid}).Decode(&tc)
	return &tc, err
}

// FindUser retrieves a user by its hex ID.
func FindUser(ctx context.Context, id string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var u User
	err = db.Col("user").FindOne(ctx, bson.M{"_id": oid}).Decode(&u)
	return &u, err
}

// FindUserByEmail retrieves a user by email address. Returns nil, nil if not found.
func FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := db.Col("user").FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &u, err
}

// FindUserByUsername retrieves a user by username. Returns nil, nil if not found.
func FindUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := db.Col("user").FindOne(ctx, bson.M{"username": username}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &u, err
}

// FindRole retrieves a role by name.
func FindRole(ctx context.Context, name string) (*Role, error) {
	var r Role
	err := db.Col("role").FindOne(ctx, bson.M{"name": name}).Decode(&r)
	return &r, err
}

// GetTestCases returns all testcases for a given assessment ID string.
func GetTestCases(ctx context.Context, assessmentID string) ([]TestCase, error) {
	var tcs []TestCase
	cursor, err := db.Col("test_case").Find(ctx, bson.M{"assessmentid": assessmentID})
	if err != nil {
		return nil, err
	}
	if err := cursor.All(ctx, &tcs); err != nil {
		return nil, err
	}
	return tcs, nil
}

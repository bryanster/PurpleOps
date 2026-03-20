package models

import (
	"context"
	"errors"

	"github.com/bryanster/purpleops/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// FindAPIKeyByHash retrieves an API key record by its SHA-256 hash.
func FindAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialised")
	}
	var k APIKey
	err := db.Col("api_key").FindOne(ctx, bson.M{"key_hash": hash, "active": true}).Decode(&k)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &k, err
}

// FindAPIKeysByUser retrieves all API keys belonging to a user.
func FindAPIKeysByUser(ctx context.Context, userID bson.ObjectID) ([]APIKey, error) {
	var keys []APIKey
	cursor, err := db.Col("api_key").Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

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
	if db.DB == nil {
		return nil, errors.New("database not initialised")
	}
	var r Role
	err := db.Col("role").FindOne(ctx, bson.M{"name": name}).Decode(&r)
	return &r, err
}

// FindOrCreateSSOUser looks up a user by email. If found, returns it.
// If not found and autoProvision is true, creates a new user with the given
// provider and default role, then returns it.
func FindOrCreateSSOUser(ctx context.Context, email, username, provider, defaultRole string, autoProvision bool) (*User, error) {
	user, err := FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	if !autoProvision {
		return nil, nil
	}

	// Resolve default role.
	var roleIDs []bson.ObjectID
	role, err := FindRole(ctx, defaultRole)
	if err == nil && role != nil {
		roleIDs = append(roleIDs, role.ID)
	}

	if username == "" {
		username = email
	}

	newUser := User{
		ID:           bson.NewObjectID(),
		Email:        email,
		Username:     username,
		Password:     "", // SSO users have no local password
		Roles:        roleIDs,
		Active:       true,
		InitPwd:      false,
		AuthProvider: provider,
	}

	if _, err := db.Col("user").InsertOne(ctx, &newUser); err != nil {
		return nil, err
	}

	return &newUser, nil
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

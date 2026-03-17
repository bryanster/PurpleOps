package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// --- YAML structs ---

type SigmaRule struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

type ARTFile struct {
	AttackTechnique string    `yaml:"attack_technique"`
	AtomicTests     []ARTTest `yaml:"atomic_tests"`
}

type ARTTest struct {
	Name           string                 `yaml:"name"`
	Description    string                 `yaml:"description"`
	Executor       ARTExecutor            `yaml:"executor"`
	InputArguments map[string]ARTArgument `yaml:"input_arguments"`
}

type ARTExecutor struct {
	Command string `yaml:"command"`
}

type ARTArgument struct {
	Default interface{} `yaml:"default"`
}

type CustomTestcase struct {
	Name      string `yaml:"name"`
	MitreID   string `yaml:"mitreid"`
	Tactic    string `yaml:"tactic"`
	Objective string `yaml:"objective"`
	Actions   string `yaml:"actions"`
	Provider  string `yaml:"provider"`
}

type CustomKB struct {
	MitreID  string `yaml:"mitreid"`
	Overview string `yaml:"overview"`
	Advice   string `yaml:"advice"`
	Provider string `yaml:"provider"`
}

func main() {
	_ = godotenv.Load()

	mongoHost := envOrDefault("MONGO_HOST", "mongodb")
	mongoPort := envOrDefault("MONGO_PORT", "27017")
	mongoDB := envOrDefault("MONGO_DB", "purpleops")

	uri := fmt.Sprintf("mongodb://%s:%s", mongoHost, mongoPort)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	db := client.Database(mongoDB)

	// Check if tactic collection is empty (first run detection)
	count, err := db.Collection("tactic").CountDocuments(context.Background(), bson.D{})
	if err != nil {
		log.Fatalf("Failed to count tactics: %v", err)
	}
	if count > 0 {
		fmt.Println("Database already seeded, skipping.")
		return
	}

	fmt.Println("First run detected. Seeding database...")

	seedTactics(db)
	seedTechniques(db)
	seedSigma(db)
	seedART(db)
	seedCustomTestcases(db)
	seedCustomKB(db)
	seedRoles(db)
	seedAdmin(db)
	generateSecrets()

	fmt.Println("Seeding complete.")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func seedTactics(db *mongo.Database) {
	fmt.Println("Pulling MITRE tactics...")

	url := "https://github.com/CyberCX-STA/PurpleOps-Deps/raw/master/attack.mitre/15.1/enterprise-attack-v15.1-tactics.xlsx"
	tmpFile := "/tmp/tactics.xlsx"
	if err := downloadFile(url, tmpFile); err != nil {
		log.Fatalf("Failed to download tactics XLSX: %v", err)
	}
	defer os.Remove(tmpFile)

	f, err := excelize.OpenFile(tmpFile)
	if err != nil {
		log.Fatalf("Failed to open tactics XLSX: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Fatal("No sheets found in tactics XLSX")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		log.Fatalf("Failed to get rows from tactics XLSX: %v", err)
	}

	coll := db.Collection("tactic")
	var docs []interface{}
	for i, row := range rows {
		if i == 0 {
			continue // skip header
		}
		if len(row) < 2 {
			continue
		}
		docs = append(docs, bson.D{
			{Key: "mitre_id", Value: row[0]},
			{Key: "name", Value: row[1]},
		})
	}
	if len(docs) > 0 {
		if _, err := coll.InsertMany(context.Background(), docs); err != nil {
			log.Fatalf("Failed to insert tactics: %v", err)
		}
	}
	fmt.Printf("  Inserted %d tactics\n", len(docs))
}

func seedTechniques(db *mongo.Database) {
	fmt.Println("Pulling MITRE techniques...")

	url := "https://github.com/CyberCX-STA/PurpleOps-Deps/raw/master/attack.mitre/15.1/enterprise-attack-v15.1-techniques.xlsx"
	tmpFile := "/tmp/techniques.xlsx"
	if err := downloadFile(url, tmpFile); err != nil {
		log.Fatalf("Failed to download techniques XLSX: %v", err)
	}
	defer os.Remove(tmpFile)

	f, err := excelize.OpenFile(tmpFile)
	if err != nil {
		log.Fatalf("Failed to open techniques XLSX: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Fatal("No sheets found in techniques XLSX")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		log.Fatalf("Failed to get rows from techniques XLSX: %v", err)
	}

	techColl := db.Collection("technique")
	kbColl := db.Collection("knowlege_base") // typo preserved from original

	var techDocs []interface{}
	var kbDocs []interface{}

	for i, row := range rows {
		if i == 0 {
			continue // skip header
		}
		if len(row) < 2 {
			continue
		}

		mitreID := row[0]
		name := row[1]
		description := ""
		detection := ""
		tacticsStr := ""

		if len(row) > 2 {
			description = row[2]
		}
		if len(row) > 3 {
			detection = row[3]
		}
		if len(row) > 4 {
			tacticsStr = row[4]
		}

		var tactics []string
		if tacticsStr != "" {
			for _, t := range strings.Split(tacticsStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tactics = append(tactics, t)
				}
			}
		}

		techDocs = append(techDocs, bson.D{
			{Key: "mitre_id", Value: mitreID},
			{Key: "name", Value: name},
			{Key: "description", Value: description},
			{Key: "detection", Value: detection},
			{Key: "tactics", Value: tactics},
		})

		kbDocs = append(kbDocs, bson.D{
			{Key: "mitre_id", Value: mitreID},
			{Key: "name", Value: name},
			{Key: "overview", Value: description},
			{Key: "advice", Value: detection},
			{Key: "provider", Value: "MITRE ATT&CK"},
		})
	}

	if len(techDocs) > 0 {
		if _, err := techColl.InsertMany(context.Background(), techDocs); err != nil {
			log.Fatalf("Failed to insert techniques: %v", err)
		}
	}
	fmt.Printf("  Inserted %d techniques\n", len(techDocs))

	if len(kbDocs) > 0 {
		if _, err := kbColl.InsertMany(context.Background(), kbDocs); err != nil {
			log.Fatalf("Failed to insert knowledge base entries: %v", err)
		}
	}
	fmt.Printf("  Inserted %d knowledge base entries\n", len(kbDocs))
}

func seedSigma(db *mongo.Database) {
	fmt.Println("Cloning SigmaHQ/sigma repository...")

	cloneDir := "/tmp/sigma"
	os.RemoveAll(cloneDir)

	_, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL: "https://github.com/SigmaHQ/sigma.git",
	})
	if err != nil {
		log.Fatalf("Failed to clone sigma repo: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	fmt.Println("Parsing Sigma rules...")
	coll := db.Collection("sigma")
	var docs []interface{}

	rulesDir := filepath.Join(cloneDir, "rules")
	err = filepath.Walk(rulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yml") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var rule SigmaRule
		if err := yaml.Unmarshal(data, &rule); err != nil {
			return nil
		}

		var mitreIDs []string
		for _, tag := range rule.Tags {
			tag = strings.TrimSpace(tag)
			if strings.HasPrefix(tag, "attack.t") || strings.HasPrefix(tag, "attack.T") {
				id := strings.TrimPrefix(tag, "attack.")
				id = strings.ToUpper(id)
				mitreIDs = append(mitreIDs, id)
			}
		}

		if len(mitreIDs) == 0 {
			return nil
		}

		for _, mitreID := range mitreIDs {
			docs = append(docs, bson.D{
				{Key: "mitre_id", Value: mitreID},
				{Key: "title", Value: rule.Title},
				{Key: "description", Value: rule.Description},
			})
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk sigma rules: %v", err)
	}

	if len(docs) > 0 {
		if _, err := coll.InsertMany(context.Background(), docs); err != nil {
			log.Fatalf("Failed to insert sigma rules: %v", err)
		}
	}
	fmt.Printf("  Inserted %d sigma rules\n", len(docs))
}

func seedART(db *mongo.Database) {
	fmt.Println("Cloning redcanaryco/atomic-red-team repository...")

	cloneDir := "/tmp/atomic-red-team"
	os.RemoveAll(cloneDir)

	_, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL: "https://github.com/redcanaryco/atomic-red-team.git",
	})
	if err != nil {
		log.Fatalf("Failed to clone atomic-red-team repo: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	fmt.Println("Parsing Atomic Red Team testcases...")
	coll := db.Collection("test_case_template")
	var docs []interface{}

	atomicsDir := filepath.Join(cloneDir, "atomics")
	entries, err := os.ReadDir(atomicsDir)
	if err != nil {
		log.Fatalf("Failed to read atomics dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "T") {
			continue
		}

		dirPath := filepath.Join(atomicsDir, entry.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".yaml") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, f.Name()))
			if err != nil {
				continue
			}

			var artFile ARTFile
			if err := yaml.Unmarshal(data, &artFile); err != nil {
				continue
			}

			for _, test := range artFile.AtomicTests {
				// Build default args string
				var argParts []string
				for name, arg := range test.InputArguments {
					argParts = append(argParts, fmt.Sprintf("%s: %v", name, arg.Default))
				}

				docs = append(docs, bson.D{
					{Key: "mitre_id", Value: artFile.AttackTechnique},
					{Key: "name", Value: test.Name},
					{Key: "description", Value: test.Description},
					{Key: "command", Value: test.Executor.Command},
					{Key: "arguments", Value: strings.Join(argParts, "\n")},
					{Key: "provider", Value: "Atomic Red Team"},
				})
			}
		}
	}

	if len(docs) > 0 {
		if _, err := coll.InsertMany(context.Background(), docs); err != nil {
			log.Fatalf("Failed to insert ART testcases: %v", err)
		}
	}
	fmt.Printf("  Inserted %d ART test case templates\n", len(docs))
}

func seedCustomTestcases(db *mongo.Database) {
	fmt.Println("Parsing custom testcases...")

	coll := db.Collection("test_case_template")
	var docs []interface{}

	files, err := filepath.Glob("custom/testcases/*.yaml")
	if err != nil || len(files) == 0 {
		fmt.Println("  No custom testcases found")
		return
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		var tc CustomTestcase
		if err := yaml.Unmarshal(data, &tc); err != nil {
			continue
		}

		docs = append(docs, bson.D{
			{Key: "mitre_id", Value: tc.MitreID},
			{Key: "name", Value: tc.Name},
			{Key: "tactic", Value: tc.Tactic},
			{Key: "objective", Value: tc.Objective},
			{Key: "actions", Value: tc.Actions},
			{Key: "provider", Value: tc.Provider},
		})
	}

	if len(docs) > 0 {
		if _, err := coll.InsertMany(context.Background(), docs); err != nil {
			log.Fatalf("Failed to insert custom testcases: %v", err)
		}
	}
	fmt.Printf("  Inserted %d custom test case templates\n", len(docs))
}

func seedCustomKB(db *mongo.Database) {
	fmt.Println("Parsing custom knowledge bases...")

	coll := db.Collection("knowlege_base") // typo preserved from original
	var docs []interface{}

	files, err := filepath.Glob("custom/knowledgebase/*.yaml")
	if err != nil || len(files) == 0 {
		fmt.Println("  No custom knowledge bases found")
		return
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		var kb CustomKB
		if err := yaml.Unmarshal(data, &kb); err != nil {
			continue
		}

		docs = append(docs, bson.D{
			{Key: "mitre_id", Value: kb.MitreID},
			{Key: "overview", Value: kb.Overview},
			{Key: "advice", Value: kb.Advice},
			{Key: "provider", Value: kb.Provider},
		})
	}

	if len(docs) > 0 {
		if _, err := coll.InsertMany(context.Background(), docs); err != nil {
			log.Fatalf("Failed to insert custom KBs: %v", err)
		}
	}
	fmt.Printf("  Inserted %d custom knowledge base entries\n", len(docs))
}

func seedRoles(db *mongo.Database) {
	fmt.Println("Creating roles...")

	coll := db.Collection("role")
	roles := []interface{}{
		bson.D{{Key: "name", Value: "Admin"}},
		bson.D{{Key: "name", Value: "Red"}},
		bson.D{{Key: "name", Value: "Blue"}},
		bson.D{{Key: "name", Value: "Spectator"}},
	}

	if _, err := coll.InsertMany(context.Background(), roles); err != nil {
		log.Fatalf("Failed to insert roles: %v", err)
	}
	fmt.Printf("  Inserted %d roles\n", len(roles))
}

func seedAdmin(db *mongo.Database) {
	fmt.Println("Creating admin user...")

	// Get Admin role ID
	roleColl := db.Collection("role")
	var role bson.M
	err := roleColl.FindOne(context.Background(), bson.D{{Key: "name", Value: "Admin"}}).Decode(&role)
	if err != nil {
		log.Fatalf("Failed to find Admin role: %v", err)
	}

	password := generateUUID()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	userColl := db.Collection("user")
	_, err = userColl.InsertOne(context.Background(), bson.D{
		{Key: "email", Value: "admin@purpleops.com"},
		{Key: "username", Value: "admin"},
		{Key: "password", Value: string(hashedPassword)},
		{Key: "roles", Value: bson.A{role["_id"]}},
		{Key: "active", Value: true},
	})
	if err != nil {
		log.Fatalf("Failed to insert admin user: %v", err)
	}

	fmt.Println("  Admin user created:")
	fmt.Printf("    Email:    admin@purpleops.com\n")
	fmt.Printf("    Username: admin\n")
	fmt.Printf("    Password: %s\n", password)
}

func generateSecrets() {
	fmt.Println("Generating secrets...")

	envPath := ".env"
	content := ""

	// Read existing .env if it exists
	if data, err := os.ReadFile(envPath); err == nil {
		content = string(data)
	}

	secretKey := randomHex(32)
	securitySalt := randomHex(16)

	// Append secrets if not already present
	if !strings.Contains(content, "SECRET_KEY=") {
		content += fmt.Sprintf("\nSECRET_KEY=%s", secretKey)
	}
	if !strings.Contains(content, "SECURITY_PASSWORD_SALT=") {
		content += fmt.Sprintf("\nSECURITY_PASSWORD_SALT=%s", securitySalt)
	}

	if err := os.WriteFile(envPath, []byte(strings.TrimSpace(content)+"\n"), 0644); err != nil {
		log.Fatalf("Failed to write .env: %v", err)
	}

	fmt.Println("  Secrets written to .env")
}

func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate UUID: %v", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate random bytes: %v", err)
	}
	return hex.EncodeToString(b)
}

// Package embedding provides face embedding storage and retrieval
package embedding

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// User represents an enrolled user
type User struct {
	ID         string      `json:"id"`
	Username   string      `json:"username"`
	Embeddings [][]float32 `json:"embeddings"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	LastUsedAt *time.Time  `json:"last_used_at,omitempty"`
	UseCount   int         `json:"use_count"`
	Active     bool        `json:"active"`
}

// Store provides persistent storage for face embeddings
type Store struct {
	db      *sql.DB
	dataDir string
}

// NewStore creates a new embedding store
func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:      db,
		dataDir: dir,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database tables
func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		embeddings BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		use_count INTEGER DEFAULT 0,
		active BOOLEAN DEFAULT 1
	);
	
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);
	
	CREATE TABLE IF NOT EXISTS auth_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		username TEXT,
		success BOOLEAN NOT NULL,
		confidence REAL,
		liveness_passed BOOLEAN,
		challenge_passed BOOLEAN,
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	
	CREATE INDEX IF NOT EXISTS idx_auth_logs_user_id ON auth_logs(user_id);
	CREATE INDEX IF NOT EXISTS idx_auth_logs_created_at ON auth_logs(created_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// CreateUser creates a new user with embeddings
func (s *Store) CreateUser(username string, embeddings [][]float32) (*User, error) {
	// Generate ID from username hash
	hash := sha256.Sum256([]byte(username))
	id := hex.EncodeToString(hash[:16])

	// Serialize embeddings
	embeddingsJSON, err := json.Marshal(embeddings)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize embeddings: %w", err)
	}

	now := time.Now()

	_, err = s.db.Exec(
		`INSERT INTO users (id, username, embeddings, created_at, updated_at) 
		 VALUES (?, ?, ?, ?, ?)`,
		id, username, embeddingsJSON, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		ID:         id,
		Username:   username,
		Embeddings: embeddings,
		CreatedAt:  now,
		UpdatedAt:  now,
		Active:     true,
	}, nil
}

// GetUser retrieves a user by username
func (s *Store) GetUser(username string) (*User, error) {
	var user User
	var embeddingsJSON []byte
	var lastUsedAt sql.NullTime

	err := s.db.QueryRow(
		`SELECT id, username, embeddings, created_at, updated_at, last_used_at, use_count, active 
		 FROM users WHERE username = ?`,
		username,
	).Scan(
		&user.ID, &user.Username, &embeddingsJSON,
		&user.CreatedAt, &user.UpdatedAt, &lastUsedAt,
		&user.UseCount, &user.Active,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastUsedAt.Valid {
		user.LastUsedAt = &lastUsedAt.Time
	}

	// Deserialize embeddings
	if err := json.Unmarshal(embeddingsJSON, &user.Embeddings); err != nil {
		return nil, fmt.Errorf("failed to deserialize embeddings: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *Store) GetUserByID(id string) (*User, error) {
	var user User
	var embeddingsJSON []byte
	var lastUsedAt sql.NullTime

	err := s.db.QueryRow(
		`SELECT id, username, embeddings, created_at, updated_at, last_used_at, use_count, active 
		 FROM users WHERE id = ?`,
		id,
	).Scan(
		&user.ID, &user.Username, &embeddingsJSON,
		&user.CreatedAt, &user.UpdatedAt, &lastUsedAt,
		&user.UseCount, &user.Active,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastUsedAt.Valid {
		user.LastUsedAt = &lastUsedAt.Time
	}

	// Deserialize embeddings
	if err := json.Unmarshal(embeddingsJSON, &user.Embeddings); err != nil {
		return nil, fmt.Errorf("failed to deserialize embeddings: %w", err)
	}

	return &user, nil
}

// UpdateUser updates a user's embeddings
func (s *Store) UpdateUser(username string, embeddings [][]float32) error {
	// Serialize embeddings
	embeddingsJSON, err := json.Marshal(embeddings)
	if err != nil {
		return fmt.Errorf("failed to serialize embeddings: %w", err)
	}

	now := time.Now()

	result, err := s.db.Exec(
		`UPDATE users SET embeddings = ?, updated_at = ? WHERE username = ?`,
		embeddingsJSON, now, username,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// DeleteUser deletes a user
func (s *Store) DeleteUser(username string) error {
	result, err := s.db.Exec(`DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// ListUsers returns all enrolled users
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, embeddings, created_at, updated_at, last_used_at, use_count, active 
		 FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var user User
		var embeddingsJSON []byte
		var lastUsedAt sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Username, &embeddingsJSON,
			&user.CreatedAt, &user.UpdatedAt, &lastUsedAt,
			&user.UseCount, &user.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		if lastUsedAt.Valid {
			user.LastUsedAt = &lastUsedAt.Time
		}

		// Deserialize embeddings
		if err := json.Unmarshal(embeddingsJSON, &user.Embeddings); err != nil {
			return nil, fmt.Errorf("failed to deserialize embeddings: %w", err)
		}

		users = append(users, user)
	}

	return users, rows.Err()
}

// RecordAuth records an authentication attempt
func (s *Store) RecordAuth(userID, username string, success bool, confidence float64,
	livenessPassed, challengePassed bool, errorMsg string) error {

	_, err := s.db.Exec(
		`INSERT INTO auth_logs (user_id, username, success, confidence, liveness_passed, challenge_passed, error_message) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, username, success, confidence, livenessPassed, challengePassed, errorMsg,
	)
	if err != nil {
		return fmt.Errorf("failed to record auth: %w", err)
	}

	// Update user stats on success
	if success && userID != "" {
		_, _ = s.db.Exec(
			`UPDATE users SET last_used_at = ?, use_count = use_count + 1 WHERE id = ?`,
			time.Now(), userID,
		)
	}

	return nil
}

// GetAuthHistory returns authentication history for a user
func (s *Store) GetAuthHistory(username string, limit int) ([]AuthLog, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, username, success, confidence, liveness_passed, challenge_passed, 
		        error_message, created_at 
		 FROM auth_logs 
		 WHERE username = ? 
		 ORDER BY created_at DESC 
		 LIMIT ?`,
		username, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var logs []AuthLog
	for rows.Next() {
		var log AuthLog
		var errorMsg sql.NullString

		err := rows.Scan(
			&log.ID, &log.UserID, &log.Username, &log.Success,
			&log.Confidence, &log.LivenessPassed, &log.ChallengePassed,
			&errorMsg, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan auth log: %w", err)
		}

		if errorMsg.Valid {
			log.ErrorMessage = errorMsg.String
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// AuthLog represents an authentication log entry
type AuthLog struct {
	ID              int64     `json:"id"`
	UserID          *string   `json:"user_id,omitempty"`
	Username        string    `json:"username"`
	Success         bool      `json:"success"`
	Confidence      float64   `json:"confidence"`
	LivenessPassed  bool      `json:"liveness_passed"`
	ChallengePassed bool      `json:"challenge_passed"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// FindBestMatch finds the best matching user for an embedding
func (s *Store) FindBestMatch(embedding []float32, threshold float64) (*User, float64, error) {
	users, err := s.ListUsers()
	if err != nil {
		return nil, 0, err
	}

	var bestUser *User
	var bestScore float64 = -1

	for i := range users {
		if !users[i].Active {
			continue
		}

		// Compare against all embeddings for this user
		for _, userEmbedding := range users[i].Embeddings {
			score := CosineSimilarity(embedding, userEmbedding)
			if score > bestScore {
				bestScore = score
				bestUser = &users[i]
			}
		}
	}

	if bestUser == nil || bestScore < threshold {
		return nil, bestScore, nil
	}

	return bestUser, bestScore, nil
}

// CosineSimilarity computes cosine similarity between two embeddings
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}

// Package embedding provides tests for the embedding store
package embedding

import (
	"os"
	"testing"
)

func TestStore(t *testing.T) {
	// Create temporary database
	dbPath := "/tmp/test_facelock.db"
	defer func() { _ = os.Remove(dbPath) }()

	// Create store
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Test user creation
	t.Run("CreateUser", func(t *testing.T) {
		embeddings := [][]float32{
			{0.1, 0.2, 0.3, 0.4, 0.5},
			{0.2, 0.3, 0.4, 0.5, 0.6},
		}

		user, err := store.CreateUser("testuser", embeddings)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", user.Username)
		}

		if len(user.Embeddings) != 2 {
			t.Errorf("Expected 2 embeddings, got %d", len(user.Embeddings))
		}
	})

	// Test get user
	t.Run("GetUser", func(t *testing.T) {
		user, err := store.GetUser("testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", user.Username)
		}
	})

	// Test find best match
	t.Run("FindBestMatch", func(t *testing.T) {
		// Create a query embedding similar to the first user embedding
		queryEmbedding := []float32{0.11, 0.21, 0.31, 0.41, 0.51}

		user, confidence, err := store.FindBestMatch(queryEmbedding, 0.5)
		if err != nil {
			t.Fatalf("Failed to find best match: %v", err)
		}

		if user == nil {
			t.Error("Expected to find a match, got nil")
		} else if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", user.Username)
		}

		t.Logf("Match confidence: %.3f", confidence)
	})

	// Test list users
	t.Run("ListUsers", func(t *testing.T) {
		users, err := store.ListUsers()
		if err != nil {
			t.Fatalf("Failed to list users: %v", err)
		}

		if len(users) != 1 {
			t.Errorf("Expected 1 user, got %d", len(users))
		}
	})

	// Test delete user
	t.Run("DeleteUser", func(t *testing.T) {
		err := store.DeleteUser("testuser")
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		// Verify user is deleted
		_, err = store.GetUser("testuser")
		if err == nil {
			t.Error("Expected error when getting deleted user, got nil")
		}
	})
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		a         []float32
		b         []float32
		expected  float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{1, 0, 0},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "orthogonal vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{0, 1, 0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "opposite vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{-1, 0, 0},
			expected:  -1.0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("Expected %.3f, got %.3f (diff: %.3f)", tt.expected, result, diff)
			}
		})
	}
}

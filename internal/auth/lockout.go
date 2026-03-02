// Package auth provides lockout mechanism
package auth

import (
	"fmt"
	"time"
)

// CheckLockout checks if a user is currently locked out
func (e *Engine) CheckLockout(username string) error {
	maxAttempts := e.config.Auth.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	lockoutDuration := time.Duration(e.config.Auth.LockoutDuration) * time.Second
	if lockoutDuration <= 0 {
		lockoutDuration = 5 * time.Minute
	}

	windowStart := time.Now().Add(-lockoutDuration)
	if e.embeddingStore != nil {
		lastSuccess, err := e.embeddingStore.GetLastSuccessTime(username)
		if err != nil {
			e.logger.Warnf("Failed to read last success for lockout check (%s): %v", username, err)
		} else if lastSuccess != nil && lastSuccess.After(windowStart) {
			windowStart = *lastSuccess
		}

		failureCount, err := e.embeddingStore.CountFailuresSince(username, windowStart)
		if err != nil {
			e.logger.Warnf("Failed to count auth failures for lockout check (%s): %v", username, err)
		} else if failureCount >= maxAttempts {
			lastFailure, failureErr := e.embeddingStore.GetLastFailureTimeSince(username, windowStart)
			if failureErr != nil {
				e.logger.Warnf("Failed to read last failure for lockout check (%s): %v", username, failureErr)
			} else if lastFailure != nil {
				lockedUntil := lastFailure.Add(lockoutDuration)
				if time.Now().Before(lockedUntil) {
					remainingTime := time.Until(lockedUntil)
					return fmt.Errorf("%w: locked for %v", ErrAccountLocked, remainingTime.Round(time.Second))
				}
			}
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	tracker, exists := e.failedAttempts[username]
	if !exists {
		return nil
	}

	// Check if user is currently locked out
	if time.Now().Before(tracker.LockedUntil) {
		remainingTime := time.Until(tracker.LockedUntil)
		return fmt.Errorf("%w: locked for %v", ErrAccountLocked, remainingTime.Round(time.Second))
	}

	return nil
}

// RecordFailure records a failed authentication attempt
func (e *Engine) RecordFailure(username string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tracker, exists := e.failedAttempts[username]
	if !exists {
		tracker = &FailureTracker{}
		e.failedAttempts[username] = tracker
	}

	tracker.Count++
	tracker.LastAttempt = time.Now()

	// Check if we need to lock out the user
	maxAttempts := e.config.Auth.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3 // Default
	}

	if tracker.Count >= maxAttempts {
		lockoutDuration := time.Duration(e.config.Auth.LockoutDuration) * time.Second
		if lockoutDuration == 0 {
			lockoutDuration = 5 * time.Minute // Default 5 minutes
		}

		tracker.LockedUntil = time.Now().Add(lockoutDuration)
		e.logger.Warnf("User %s locked out for %v after %d failed attempts",
			username, lockoutDuration, tracker.Count)
	}
}

// RecordSuccess records a successful authentication and clears failures
func (e *Engine) RecordSuccess(username string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear failed attempts on successful auth
	delete(e.failedAttempts, username)
}

// ClearLockout clears lockout for a user (admin function)
func (e *Engine) ClearLockout(username string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.failedAttempts, username)
	e.logger.Infof("Lockout cleared for user %s", username)
}

// CleanupExpiredLockouts removes old lockout entries (should be called periodically)
func (e *Engine) CleanupExpiredLockouts() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for username, tracker := range e.failedAttempts {
		// Remove if lockout expired and no recent attempts
		if now.After(tracker.LockedUntil) && now.Sub(tracker.LastAttempt) > 1*time.Hour {
			delete(e.failedAttempts, username)
		}
	}
}

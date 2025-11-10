package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

// Domain types - independent of gRPC/protobuf
type PaginationParams struct {
	PageSize int
	Token    int
}

type Liker struct {
	ActorID       string
	UnixTimestamp uint64
}

type ListLikedYouResult struct {
	Likers              []Liker
	NextPaginationToken string
}

type ExploreBusiness struct {
	db *DB
}

// NewExploreBusiness creates a new business logic service
func NewExploreBusiness(db *DB) *ExploreBusiness {
	return &ExploreBusiness{db: db}
}

// parsePaginationParams extracts and validates pagination parameters
// This is centralized to avoid duplication
func parsePaginationParams(pageSize *uint32, token *string) (PaginationParams, error) {
	const (
		defaultPageSize        = 2
		defaultPaginationToken = 0
	)

	params := PaginationParams{
		PageSize: defaultPageSize,
		Token:    defaultPaginationToken,
	}

	if pageSize != nil && *pageSize > 0 {
		params.PageSize = int(*pageSize)
	}

	if token != nil && *token != "" {
		parsedToken, err := strconv.Atoi(*token)
		if err != nil {
			return params, fmt.Errorf("invalid pagination token %s: %w", *token, err)
		}
		if parsedToken > 0 {
			params.Token = parsedToken
		}
	}

	return params, nil
}

// ListLikedYouUsers returns all users who liked the recipient
// This is the business logic - it works with domain types, not protobuf
func (b *ExploreBusiness) ListLikedYouUsers(ctx context.Context, recipientID string, pagination PaginationParams) (*ListLikedYouResult, error) {
	const query = `
		SELECT 
			id,
			actor_user_id,
			UNIX_TIMESTAMP(created_at) 
		FROM decision 
		WHERE recipient_user_id = ?
			AND liked_recipient = true
			AND id > ?
		ORDER BY id ASC
		LIMIT ?;
	`

	result, err := b.db.QueryContext(ctx, query, recipientID, pagination.Token, pagination.PageSize)
	if err != nil {
		return nil, fmt.Errorf("error querying liked users: %w", err)
	}
	defer result.Close()

	var likers []Liker
	var lastId uint64
	for result.Next() {
		var liker Liker
		if err := result.Scan(&lastId, &liker.ActorID, &liker.UnixTimestamp); err != nil {
			return nil, fmt.Errorf("error scanning liked user: %w", err)
		}
		likers = append(likers, liker)
	}

	var nextPaginationToken string
	if len(likers) == pagination.PageSize {
		nextPaginationToken = fmt.Sprintf("%d", lastId)
	}

	return &ListLikedYouResult{
		Likers:              likers,
		NextPaginationToken: nextPaginationToken,
	}, nil
}

// ListNewLikedYouUsers returns users who liked the recipient, excluding mutual likes
func (b *ExploreBusiness) ListNewLikedYouUsers(ctx context.Context, recipientID string, pagination PaginationParams) (*ListLikedYouResult, error) {
	const query = `
		SELECT
			d.id,
			d.actor_user_id, 
			UNIX_TIMESTAMP(d.created_at)
		FROM decision d
		WHERE 
			d.recipient_user_id = ?
			AND d.liked_recipient = TRUE
			AND d.id > ?
			AND NOT EXISTS (
				SELECT 1 
				FROM decision d2
				WHERE 
					d2.actor_user_id = ?
					AND d2.recipient_user_id = d.actor_user_id
					AND d2.liked_recipient = TRUE
			)
		ORDER BY d.id ASC
		LIMIT ?;
	`

	result, err := b.db.QueryContext(ctx, query, recipientID, pagination.Token, recipientID, pagination.PageSize)
	if err != nil {
		return nil, fmt.Errorf("error querying new liked users: %w", err)
	}
	defer result.Close()

	var likers []Liker
	var lastId uint64
	for result.Next() {
		var liker Liker
		if err := result.Scan(&lastId, &liker.ActorID, &liker.UnixTimestamp); err != nil {
			return nil, fmt.Errorf("error scanning new liked user: %w", err)
		}
		likers = append(likers, liker)
	}

	var nextPaginationToken string
	if len(likers) == pagination.PageSize {
		nextPaginationToken = fmt.Sprintf("%d", lastId)
	}

	return &ListLikedYouResult{
		Likers:              likers,
		NextPaginationToken: nextPaginationToken,
	}, nil
}

// CountLikedYouUsers returns the count of users who liked the recipient
func (b *ExploreBusiness) CountLikedYouUsers(ctx context.Context, recipientID string) (uint64, error) {
	const query = `
		SELECT 
			like_count
		FROM like_stats
		WHERE user_id = ?;
	`

	var count uint64
	err := b.db.QueryRowContext(ctx, query, recipientID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error getting likes count for id %s: %w", recipientID, err)
	}

	return count, nil
}

// RecordDecision records a user's decision (like/pass) and updates statistics
// This method handles all the complex business logic including:
// - Transaction management
// - Determining if counters should increment/decrement
// - Checking for mutual likes
func (b *ExploreBusiness) RecordDecision(ctx context.Context, actorID, recipientID string, likedRecipient bool) (bool, error) {
	// Start transaction for atomicity
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("error beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Check if previous decision exists
	var previousLike sql.NullBool
	const decisionExistQuery = `
		SELECT
			liked_recipient
		FROM decision
		WHERE actor_user_id = ?
			AND recipient_user_id = ?
	`
	err = tx.QueryRowContext(ctx, decisionExistQuery, actorID, recipientID).Scan(&previousLike)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("error getting previous decision (%s -> %s): %w", actorID, recipientID, err)
	}

	// 2. Determine if we should update like_stats
	shouldIncrementLikeCounter := false
	shouldDecrementLikeCounter := false
	if !previousLike.Valid {
		// No previous decision exists
		if likedRecipient {
			shouldIncrementLikeCounter = true
		}
	} else {
		// Previous decision exists
		if !previousLike.Bool && likedRecipient {
			// Changed from pass to like: increment
			shouldIncrementLikeCounter = true
		} else if previousLike.Bool && !likedRecipient {
			// Changed from like to pass: decrement
			shouldDecrementLikeCounter = true
		}
	}

	// 3. Insert or update decision
	const insertQuery = `
		INSERT INTO decision (actor_user_id, recipient_user_id, liked_recipient)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			liked_recipient = VALUES(liked_recipient),
			created_at = CURRENT_TIMESTAMP;
	`
	if _, err := tx.ExecContext(ctx, insertQuery, actorID, recipientID, likedRecipient); err != nil {
		return false, fmt.Errorf("error inserting decision (%s -> %s): %w", actorID, recipientID, err)
	}

	// 4. Update like_stats if needed
	if shouldIncrementLikeCounter {
		const incQuery = `
			INSERT INTO like_stats (user_id, like_count)
			VALUES (?, 1)
			ON DUPLICATE KEY UPDATE like_count = like_count + 1;
		`
		if _, err := tx.ExecContext(ctx, incQuery, recipientID); err != nil {
			return false, fmt.Errorf("error incrementing like_count: %w", err)
		}
	} else if shouldDecrementLikeCounter {
		const decQuery = `
			UPDATE like_stats
			SET like_count = GREATEST(like_count - 1, 0)
			WHERE user_id = ?;
		`
		if _, err := tx.ExecContext(ctx, decQuery, recipientID); err != nil {
			return false, fmt.Errorf("error decrementing like_count: %w", err)
		}
	}

	// 5. Check for mutual likes (only if actor liked recipient)
	isMutual := false
	if likedRecipient {
		const mutualCheckQuery = `
			SELECT EXISTS (
				SELECT 1
				FROM decision
				WHERE actor_user_id = ?
					AND recipient_user_id = ?
					AND liked_recipient = TRUE
			) AS recipient_liked_actor;
		`

		var exists bool
		err := tx.QueryRowContext(ctx, mutualCheckQuery, recipientID, actorID).Scan(&exists)
		if err != nil {
			return false, fmt.Errorf("error checking mutual like between %s and %s: %w",
				actorID, recipientID, err)
		}
		isMutual = exists
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit failed: %w", err)
	}

	return isMutual, nil
}
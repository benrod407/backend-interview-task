package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	pb "github.com/benrod407/explore-service/explore_service_proto"
)

type ExploreService struct {
	pb.UnimplementedExploreServiceServer
	DB *DB
}

const (
	defaultPageSize        = 2
	defaultPaginationToken = 0
)

// ListLikedYou list all users who liked the recipient
func (s *ExploreService) ListLikedYou(ctx context.Context, req *pb.ListLikedYouRequest) (*pb.ListLikedYouResponse, error) {
	const query = `
		SELECT 
			id,
			actor_user_id,
			UNIX_TIMESTAMP(created_at) 
		FROM decision 
		WHERE recipient_user_id = ? 	-- recipient
			AND liked_recipient = true
			AND id > ? 					-- cursor pagination token
		ORDER BY id ASC
		LIMIT ?; 						-- page size
	`

	// Pagination logic
	pageSize := defaultPageSize
	if req.PageSize != nil && *req.PageSize > 0 {
		pageSize = int(*req.PageSize)
	}

	cursorPaginationToken := defaultPaginationToken
	if req.PaginationToken != nil && *req.PaginationToken != "" {
		token, err := strconv.Atoi(*req.PaginationToken)
		if err != nil {
			return nil, fmt.Errorf("invalid pagination token %s: %w", *req.PaginationToken, err)
		}
		if token > 0 {
			cursorPaginationToken = token
		}
	}

	// Apply query
	result, err := s.DB.QueryContext(ctx, query, req.RecipientUserId, cursorPaginationToken, pageSize)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	// Loop through query result
	var likers []*pb.ListLikedYouResponse_Liker
	var lastId uint64
	for result.Next() {
		var liker pb.ListLikedYouResponse_Liker
		if err := result.Scan(&lastId, &liker.ActorId, &liker.UnixTimestamp); err != nil {
			return nil, fmt.Errorf("error getting ListLikedYou for %s: %w", req.RecipientUserId, err)
		}
		likers = append(likers, &liker)
	}

	// When all the results have been shown the last token is an empty string
	var nextPaginationToken string
	if len(likers) == pageSize {
		nextPaginationToken = fmt.Sprintf("%d", lastId)
	}
	return &pb.ListLikedYouResponse{
		Likers:              likers,
		NextPaginationToken: &nextPaginationToken,
	}, nil
}

// ListNewLikedYou list all users who liked the recipient excluding those who have been liked in return
func (s *ExploreService) ListNewLikedYou(ctx context.Context, req *pb.ListLikedYouRequest) (*pb.ListLikedYouResponse, error) {
	const query = `
		SELECT
			d.id,
			d.actor_user_id, 
			UNIX_TIMESTAMP(d.created_at)
		FROM decision d
		WHERE 
			d.recipient_user_id = ?				-- recipient
			AND d.liked_recipient = TRUE		-- users who liked the recipient
			AND d.id > ?						-- pagination cursor
			AND NOT EXISTS (				  	-- users liked by the recipient query
				SELECT 1 
				FROM decision d2
				WHERE 
					d2.actor_user_id = ?       -- recipient
					AND d2.recipient_user_id = d.actor_user_id
					AND d2.liked_recipient = TRUE
			)
		ORDER BY d.id ASC
		LIMIT ?; 								-- page size
	`

	// Pagination logic
	pageSize := defaultPageSize
	if req.PageSize != nil && *req.PageSize > 0 {
		pageSize = int(*req.PageSize)
	}

	cursorPaginationToken := defaultPaginationToken
	if req.PaginationToken != nil && *req.PaginationToken != "" {
		token, err := strconv.Atoi(*req.PaginationToken)
		if err != nil {
			return nil, fmt.Errorf("invalid pagination token %s: %w", *req.PaginationToken, err)
		}
		if token > 0 {
			cursorPaginationToken = token
		}
	}

	// Apply query
	result, err := s.DB.QueryContext(ctx, query, req.RecipientUserId, cursorPaginationToken, req.RecipientUserId, pageSize)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	// Loop through query result
	var likers []*pb.ListLikedYouResponse_Liker
	var lastId uint64
	for result.Next() {
		var liker pb.ListLikedYouResponse_Liker
		if err := result.Scan(&lastId, &liker.ActorId, &liker.UnixTimestamp); err != nil {
			return nil, fmt.Errorf("error getting ListNewLikedYou for %s: %w", req.RecipientUserId, err)
		}
		likers = append(likers, &liker)
	}

	// When all the results have been shown the last token is an empty string
	var nextPaginationToken string
	if len(likers) == pageSize {
		nextPaginationToken = fmt.Sprintf("%d", lastId)
	}
	return &pb.ListLikedYouResponse{
		Likers:              likers,
		NextPaginationToken: &nextPaginationToken,
	}, nil
}

// CountLikedYou count the number of users who liked the recipient
func (s *ExploreService) CountLikedYou(ctx context.Context, countLikedYouRequest *pb.CountLikedYouRequest) (*pb.CountLikedYouResponse, error) {
	var count uint64
	const query = `
		SELECT 
			like_count
		FROM like_stats
		WHERE user_id = ?;
	`
	err := s.DB.QueryRowContext(ctx, query, countLikedYouRequest.RecipientUserId).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("error getting likes count for id %s: %w", countLikedYouRequest.RecipientUserId, err)
	}

	return &pb.CountLikedYouResponse{
		Count: count,
	}, nil
}

// PutDecision record an actor decision (like/pass) over a recipient,
// updates the like_stats table (only if the stored value has changed),
// and returns whether the actor/recipient have mutual likes
func (s *ExploreService) PutDecision(ctx context.Context, req *pb.PutDecisionRequest) (*pb.PutDecisionResponse, error) {
	// To guarantee consistency and atomicity we will use transactions with Begin/Commit/Rollback
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error setting transaction begin tx: %w", err)
	}
	// Defer rollback on error for queries
	defer tx.Rollback()

	// 1. Check current decision
	var previousLike sql.NullBool
	const decisionExistQuery = `
		SELECT
			liked_recipient
		FROM decision
		WHERE actor_user_id = ?
			AND recipient_user_id = ?
	`
	err = tx.QueryRowContext(ctx, decisionExistQuery, req.ActorUserId, req.RecipientUserId).Scan(&previousLike)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error getting previous decision (%s -> %s): %w", req.ActorUserId, req.RecipientUserId, err)
	}

	// 2. Should we increase/decrease like_stats entry
	shouldIncrementLikeCounter := false
	shouldDecrementLikeCounter := false
	if !previousLike.Valid { // previous decision does not exists
		if req.LikedRecipient {
			shouldIncrementLikeCounter = true
		}
	} else { // a previous decision exists
		if !previousLike.Bool && req.LikedRecipient {
			// false -> true : should increment
			shouldIncrementLikeCounter = true
		} else if previousLike.Bool && !req.LikedRecipient {
			// true -> false : should decrement
			shouldDecrementLikeCounter = true
		}
	}

	// 3. Insert new actor decision
	const insertQuery = `
		INSERT INTO decision (actor_user_id, recipient_user_id, liked_recipient)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			liked_recipient = VALUES(liked_recipient),
			created_at = CURRENT_TIMESTAMP;
	`
	if _, err := s.DB.ExecContext(ctx, insertQuery, req.ActorUserId, req.RecipientUserId, req.LikedRecipient); err != nil {
		return nil, fmt.Errorf("error inserting decision (%s -> %s): %w", req.ActorUserId, req.RecipientUserId, err)
	}

	// 4. Update like_stats only when needed
	if shouldIncrementLikeCounter {
		const incQuery = `
			INSERT INTO like_stats (user_id, like_count)
			VALUES (?, 1)
			ON DUPLICATE KEY UPDATE like_count = like_count + 1;
		`
		if _, err := s.DB.ExecContext(ctx, incQuery, req.RecipientUserId); err != nil {
			return nil, fmt.Errorf("error incrementing like_count: %w", err)
		}
	} else if shouldDecrementLikeCounter {
		const decQuery = `
			UPDATE like_stats
			SET like_count = GREATEST(like_count - 1, 0)
			WHERE user_id = ?;
		`
		if _, err := s.DB.ExecContext(ctx, decQuery, req.RecipientUserId); err != nil {
			return nil, fmt.Errorf("error decrementing like_count: %w", err)
		}
	}

	// 5. Check mutual likes
	isMutual := false
	if req.LikedRecipient {
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
		err := s.DB.QueryRowContext(ctx, mutualCheckQuery, req.RecipientUserId, req.ActorUserId).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("error checking mutual like between %s and %s: %w",
				req.ActorUserId, req.RecipientUserId, err)
		}
		isMutual = exists
	}

	// Commit db changes or rollback
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &pb.PutDecisionResponse{
		MutualLikes: isMutual,
	}, nil
}

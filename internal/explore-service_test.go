package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	pb "github.com/benrod407/explore-service/explore_service_proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *ExploreService, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "failed to create sqlmock")
	service := &ExploreService{
		Business: NewExploreBusiness(&DB{db}),
	}

	cleanup := func() {
		db.Close()
	}

	return db, mock, service, cleanup
}

func TestCountLikedYou(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT like_count`).
		WithArgs("user123").
		WillReturnRows(sqlmock.NewRows([]string{"like_count"}).AddRow(3))

	resp, err := service.CountLikedYou(context.Background(), &pb.CountLikedYouRequest{
		RecipientUserId: "user123",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, uint64(3), resp.Count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListLikedYou(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	pagination, err := parsePaginationParams(nil, nil)
	require.NoError(t, err)

	sqlRowsQueryResult := sqlmock.NewRows([]string{"id", "actor_user_id", "unix_timestamp"}).
		AddRow(1, "uuid-user-A", 1700000000).
		AddRow(2, "uuid-user-B", 1700001000)

	mock.ExpectQuery(`SELECT\s+id,\s+actor_user_id,\s+UNIX_TIMESTAMP\(created_at\)`).
		WithArgs(
			"uuid-recipient",
			pagination.Token,
			pagination.PageSize,
		).
		WillReturnRows(sqlRowsQueryResult)

	resp, err := service.ListLikedYou(context.Background(), &pb.ListLikedYouRequest{
		RecipientUserId: "uuid-recipient",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Likers, 2)
	assert.Equal(t, "uuid-user-A", resp.Likers[0].ActorId)
	assert.Equal(t, "uuid-user-B", resp.Likers[1].ActorId)
	assert.NotNil(t, resp.NextPaginationToken)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListNewLikedYou(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	pagination, err := parsePaginationParams(nil, nil)
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "actor_user_id", "unix_timestamp"}).
		AddRow(3, "uuid-user-X", 1700002000).
		AddRow(4, "uuid-user-Y", 1700003000)

	mock.ExpectQuery(`SELECT\s+d\.id,\s+d\.actor_user_id,\s+UNIX_TIMESTAMP\(d\.created_at\)`).
		WithArgs(
			"uuid-recipient-2",
			pagination.Token,
			"uuid-recipient-2",
			pagination.PageSize,
		).
		WillReturnRows(rows)

	resp, err := service.ListNewLikedYou(context.Background(), &pb.ListLikedYouRequest{
		RecipientUserId: "uuid-recipient-2",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Likers, 2)
	assert.Equal(t, "uuid-user-X", resp.Likers[0].ActorId)
	assert.Equal(t, "uuid-user-Y", resp.Likers[1].ActorId)
	assert.NotEmpty(t, *resp.NextPaginationToken)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPutDecision_MutualLike_WithoutPrevious(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()

	// Step 1: Check previous decision (no previous record)
	mock.ExpectQuery(`SELECT liked_recipient FROM decision`).
		WithArgs("actor1", "actor2").
		WillReturnError(sql.ErrNoRows)

	// Step 2: Insert new decision
	mock.ExpectExec(`INSERT INTO decision`).
		WithArgs("actor1", "actor2", true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 3: Increment like_count (first like for recipient)
	mock.ExpectExec(`INSERT INTO like_stats`).
		WithArgs("actor2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 4: Check mutual like
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("actor2", "actor1").
		WillReturnRows(sqlmock.NewRows([]string{"recipient_liked_actor"}).AddRow(true))

	mock.ExpectCommit()

	resp, err := service.PutDecision(context.Background(), &pb.PutDecisionRequest{
		ActorUserId:     "actor1",
		RecipientUserId: "actor2",
		LikedRecipient:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.MutualLikes)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPutDecision_NoMutualLike_WithoutPrevious(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()

	// Step 1: Check previous decision (no previous record)
	mock.ExpectQuery(`SELECT liked_recipient FROM decision`).
		WithArgs("actor1", "actor3").
		WillReturnError(sql.ErrNoRows)

	// Step 2: Insert new decision
	mock.ExpectExec(`INSERT INTO decision`).
		WithArgs("actor1", "actor3", true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 3: Increment like_count (first like for recipient)
	mock.ExpectExec(`INSERT INTO like_stats`).
		WithArgs("actor3").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 4: Check with no mutual like
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("actor3", "actor1").
		WillReturnRows(sqlmock.NewRows([]string{"recipient_liked_actor"}).AddRow(false))

	mock.ExpectCommit()

	resp, err := service.PutDecision(context.Background(), &pb.PutDecisionRequest{
		ActorUserId:     "actor1",
		RecipientUserId: "actor3",
		LikedRecipient:  true,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.MutualLikes)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPutDecision_Pass_WithoutPrevious(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()

	// Step 1: Check previous decision (no previous record)
	mock.ExpectQuery(`SELECT liked_recipient FROM decision`).
		WithArgs("actor4", "actor5").
		WillReturnError(sql.ErrNoRows)

	// Step 2: Insert new decision
	mock.ExpectExec(`INSERT INTO decision`).
		WithArgs("actor4", "actor5", false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Since it's a "pass" (liked_recipient = false), no like_stats update and no mutual check
	mock.ExpectCommit()

	resp, err := service.PutDecision(context.Background(), &pb.PutDecisionRequest{
		ActorUserId:     "actor4",
		RecipientUserId: "actor5",
		LikedRecipient:  false,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.MutualLikes)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPutDecision_WithPreviousDecision_PassToLike(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()

	// Step 1: Check previous decision and find it
	mock.ExpectQuery(`SELECT liked_recipient FROM decision`).
		WithArgs("actor1", "actor2").
		WillReturnRows(sqlmock.NewRows([]string{"liked_recipient"}).AddRow(false))

	// Step 2: Insert new decision
	mock.ExpectExec(`INSERT INTO decision`).
		WithArgs("actor1", "actor2", true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 3: Increment like_count (first like for recipient)
	mock.ExpectExec(`INSERT INTO like_stats`).
		WithArgs("actor2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 4: Check mutual like
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("actor2", "actor1").
		WillReturnRows(sqlmock.NewRows([]string{"recipient_liked_actor"}).AddRow(true))

	mock.ExpectCommit()

	resp, err := service.PutDecision(context.Background(), &pb.PutDecisionRequest{
		ActorUserId:     "actor1",
		RecipientUserId: "actor2",
		LikedRecipient:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.MutualLikes)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPutDecision_WithPreviousDecision_LikeToPass(t *testing.T) {
	_, mock, service, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()

	// Step 1: Check previous decision and find it
	mock.ExpectQuery(`SELECT liked_recipient FROM decision`).
		WithArgs("actor1", "actor2").
		WillReturnRows(sqlmock.NewRows([]string{"liked_recipient"}).AddRow(true))

	// Step 2: Insert new decision (pass)
	mock.ExpectExec(`INSERT INTO decision`).
		WithArgs("actor1", "actor2", false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 3: Decrement recipient like_count
	mock.ExpectExec(`UPDATE like_stats`).
		WithArgs("actor2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// No mutual like check
	mock.ExpectCommit()

	resp, err := service.PutDecision(context.Background(), &pb.PutDecisionRequest{
		ActorUserId:     "actor1",
		RecipientUserId: "actor2",
		LikedRecipient:  false,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.MutualLikes)

	require.NoError(t, mock.ExpectationsWereMet())
}

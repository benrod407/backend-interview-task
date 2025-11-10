package main

import (
	"context"
	"log"
	"time"

	pb "github.com/benrod407/explore-service/explore_service_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:9001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to gRPC server: %v", err)
	}

	defer conn.Close()

	c := pb.NewExploreServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allTestUsers := []string{
		"1", "2", "3", "4", "5", "6",
	}

	usedPageSize := uint32(2)

	log.Println("================================================================================")
	log.Println("==================== Check current total likes of all users ====================")

	checkCurrentLikeCount(ctx, allTestUsers, c)

	log.Println("=======================================================================================")
	log.Println("==================== Test ListLikeYou and ListNewLikeYou endpoints ====================")

	listLikeYouCall(ctx, "3", usedPageSize, c)

	listLikeYouCall(ctx, "5", usedPageSize, c)

	listLikeYouCall(ctx, "5", 1, c)

	listNewLikeYouCall(ctx, "1", usedPageSize, c)

	log.Println("==================================================================")
	log.Println("==================== Test CountLiked endpoint ====================")

	putDecisionCall(ctx, "5", "1", false, c)
	putDecisionCall(ctx, "6", "1", false, c)
	checkCurrentLikeCount(ctx, []string{"1"}, c)
	listLikeYouCall(ctx, "1", usedPageSize, c)
	putDecisionCall(ctx, "5", "1", true, c)
	checkCurrentLikeCount(ctx, []string{"1"}, c)
	putDecisionCall(ctx, "5", "1", true, c) // adding same like twice
	checkCurrentLikeCount(ctx, []string{"1"}, c)
	putDecisionCall(ctx, "6", "1", true, c)
	checkCurrentLikeCount(ctx, []string{"1"}, c)
	listLikeYouCall(ctx, "1", usedPageSize, c)

	log.Println("================================================================================")
	log.Println("==================== Check current total likes of all users ====================")

	checkCurrentLikeCount(ctx, allTestUsers, c)
}

func checkCurrentLikeCount(ctx context.Context, users []string, service pb.ExploreServiceClient) {
	for _, user := range users {
		totalLikes, err := service.CountLikedYou(
			ctx,
			&pb.CountLikedYouRequest{
				RecipientUserId: user,
			},
		)
		if err != nil {
			log.Fatalf("error calling function CountLikedYou: %v", err)
		}
		log.Printf("[CountLikedYou] recipient %s -> total likes %d", user, totalLikes.Count)

		time.Sleep(100 * time.Millisecond)
	}
}

func listLikeYouCall(ctx context.Context, recipientId string, pageSize uint32, service pb.ExploreServiceClient) {
	callCount := 0
	firstPaginationToken := ""
	var nextPaginationToken *string
	for {
		if nextPaginationToken == nil {
			nextPaginationToken = &firstPaginationToken
		}
		resp, err := service.ListLikedYou(
			ctx,
			&pb.ListLikedYouRequest{
				RecipientUserId: recipientId,
				PageSize:        &pageSize,
				PaginationToken: nextPaginationToken,
			},
		)
		if err != nil {
			log.Fatalf("error calling function ListLikedYou for user (%s): %v", recipientId, err)
		}
		logListLikedYouResponse("[ListLikedYou]", recipientId, resp)

		time.Sleep(50 * time.Millisecond)
		if resp.NextPaginationToken == nil || *resp.NextPaginationToken == "" || callCount > 10 {
			break
		} else {
			nextPaginationToken = resp.NextPaginationToken
			callCount++
		}
	}
	time.Sleep(50 * time.Millisecond)
}

func listNewLikeYouCall(ctx context.Context, recipientId string, pageSize uint32, service pb.ExploreServiceClient) {
	callCount := 0
	firstPaginationToken := ""
	var nextPaginationToken *string
	for {
		if nextPaginationToken == nil {
			nextPaginationToken = &firstPaginationToken
		}
		resp, err := service.ListNewLikedYou(
			ctx,
			&pb.ListLikedYouRequest{
				RecipientUserId: recipientId,
				PageSize:        &pageSize,
				PaginationToken: nextPaginationToken,
			},
		)
		if err != nil {
			log.Fatalf("error calling function ListNewLikedYou for user (%s): %v", recipientId, err)
		}
		logListLikedYouResponse("[ListNewLikedYou]", recipientId, resp)

		time.Sleep(50 * time.Millisecond)
		if resp.NextPaginationToken == nil || *resp.NextPaginationToken == "" || callCount > 10 {
			break
		} else {
			nextPaginationToken = resp.NextPaginationToken
			callCount++
		}
	}
	time.Sleep(50 * time.Millisecond)
}

func putDecisionCall(ctx context.Context, actorId string, recipientId string, decision bool, service pb.ExploreServiceClient) {
	isMatch, err := service.PutDecision(
		ctx,
		&pb.PutDecisionRequest{
			ActorUserId:     actorId,
			RecipientUserId: recipientId,
			LikedRecipient:  decision,
		},
	)
	if err != nil {
		log.Fatalf("error calling function PutDecision: %v", err)
	}
	log.Printf("[PutDecision] actor %s -> recipient %s | liked_recipient=%t | mutual_match=%t", actorId, recipientId, decision, isMatch.MutualLikes)

	time.Sleep(50 * time.Millisecond)
}

func logListLikedYouResponse(prefix, recipientId string, resp *pb.ListLikedYouResponse) {
	if len(resp.Likers) == 0 {
		log.Printf("%s recipient %s -> no likes found", prefix, recipientId)
	} else {
		log.Printf("%s recipient %s -> %d liker(s)", prefix, recipientId, len(resp.Likers))
		for idx, liker := range resp.Likers {
			log.Printf("%s   #%d actor %s at %s", prefix, idx+1, liker.ActorId, formatUnix(int64(liker.UnixTimestamp)))
		}
	}

	switch {
	case resp.NextPaginationToken == nil:
		log.Printf("%s recipient %s -> pagination complete (no token returned)", prefix, recipientId)
	case *resp.NextPaginationToken == "":
		log.Printf("%s recipient %s -> pagination complete (empty token)", prefix, recipientId)
	default:
		log.Printf("%s recipient %s -> next page token: %s", prefix, recipientId, *resp.NextPaginationToken)
	}
}

func formatUnix(ts int64) string {
	if ts == 0 {
		return "n/a"
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

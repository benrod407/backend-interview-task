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

	checkCurrentLikeCount(ctx, allTestUsers, c)

	listLikeYouCall(ctx, "3", usedPageSize, c)

	listLikeYouCall(ctx, "5", usedPageSize, c)

	listNewLikeYouCall(ctx, "5", usedPageSize, c)

	checkCurrentLikeCount(ctx, []string{"1"}, c)
	putDecisionCall(ctx, "5", "1", false, c)
	checkCurrentLikeCount(ctx, []string{"1"}, c)
	putDecisionCall(ctx, "5", "1", true, c)
	putDecisionCall(ctx, "5", "1", true, c)
	checkCurrentLikeCount(ctx, []string{"1"}, c)
	putDecisionCall(ctx, "6", "1", true, c)

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
		log.Printf("Total people who liked %s: %v", user, totalLikes.Count)

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
		log.Printf("People who liked user (%s) list: %v", recipientId, resp)

		time.Sleep(50 * time.Millisecond)
		if *resp.NextPaginationToken == "" || callCount > 10 {
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
		log.Printf("People who liked user (%s) list: %v", recipientId, resp)

		time.Sleep(50 * time.Millisecond)
		if *resp.NextPaginationToken == "" || callCount > 10 {
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
	log.Printf("New decision, actor (%s) recipient (%s) likes_recipient (%t)", actorId, recipientId, decision)
	log.Printf("Is a new match? : %v", isMatch.MutualLikes)

	time.Sleep(50 * time.Millisecond)
}

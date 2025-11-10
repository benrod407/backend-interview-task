package service

import (
	"context"

	pb "github.com/benrod407/explore-service/explore_service_proto"
)

type ExploreService struct {
	pb.UnimplementedExploreServiceServer
	Business *ExploreBusiness
}

// This file is a gRPC handler layer. It delegates any logic to explore-business.go

// ListLikedYou List all users who liked the recipient
func (s *ExploreService) ListLikedYou(ctx context.Context, req *pb.ListLikedYouRequest) (*pb.ListLikedYouResponse, error) {
	// 1. Parse pagination from gRPC request
	pagination, err := parsePaginationParams(req.PageSize, req.PaginationToken)
	if err != nil {
		return nil, err
	}

	// 2. Call business logic
	result, err := s.Business.ListLikedYouUsers(ctx, req.RecipientUserId, pagination)
	if err != nil {
		return nil, err
	}

	// 3. Convert domain types to protobuf response
	return convertListLikedYouResultToProtobuf(result), nil
}

// ListNewLikedYou List all users who liked the recipient excluding those who have been liked in return
func (s *ExploreService) ListNewLikedYou(ctx context.Context, req *pb.ListLikedYouRequest) (*pb.ListLikedYouResponse, error) {
	// 1. Parse pagination from gRPC request
	pagination, err := parsePaginationParams(req.PageSize, req.PaginationToken)
	if err != nil {
		return nil, err
	}

	// 2. Call business logic
	result, err := s.Business.ListNewLikedYouUsers(ctx, req.RecipientUserId, pagination)
	if err != nil {
		return nil, err
	}

	// 3. Convert to protobuf response
	return convertListLikedYouResultToProtobuf(result), nil
}

// CountLikedYou Count the number of users who liked the recipient
func (s *ExploreService) CountLikedYou(ctx context.Context, req *pb.CountLikedYouRequest) (*pb.CountLikedYouResponse, error) {
	// 1. Call business logic
	count, err := s.Business.CountLikedYouUsers(ctx, req.RecipientUserId)
	if err != nil {
		return nil, err
	}

	// 2. Convert to protobuf response
	return &pb.CountLikedYouResponse{
		Count: count,
	}, nil
}

// PutDecision Record the decision of the actor to like or pass the recipient
func (s *ExploreService) PutDecision(ctx context.Context, req *pb.PutDecisionRequest) (*pb.PutDecisionResponse, error) {
	// 1. Call business logic (handles all transaction and business rules)
	isMutual, err := s.Business.RecordDecision(ctx, req.ActorUserId, req.RecipientUserId, req.LikedRecipient)
	if err != nil {
		return nil, err
	}

	// 2. Convert to protobuf response
	return &pb.PutDecisionResponse{
		MutualLikes: isMutual,
	}, nil
}

// Helper function to convert domain types to protobuf
func convertListLikedYouResultToProtobuf(result *ListLikedYouResult) *pb.ListLikedYouResponse {
	var likers []*pb.ListLikedYouResponse_Liker
	for _, liker := range result.Likers {
		likers = append(likers, &pb.ListLikedYouResponse_Liker{
			ActorId:       liker.ActorID,
			UnixTimestamp: liker.UnixTimestamp,
		})
	}

	var nextToken *string
	if result.NextPaginationToken != "" {
		nextToken = &result.NextPaginationToken
	}

	return &pb.ListLikedYouResponse{
		Likers:              likers,
		NextPaginationToken: nextToken,
	}
}

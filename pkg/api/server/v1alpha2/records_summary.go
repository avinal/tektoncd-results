package server

import (
	"context"

	"github.com/tektoncd/results/pkg/api/server/v1alpha2/auth"
	"github.com/tektoncd/results/pkg/api/server/v1alpha2/lister"
	"github.com/tektoncd/results/pkg/api/server/v1alpha2/result"
	pb "github.com/tektoncd/results/proto/v1alpha2/results_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) GetRecordsListSummary(ctx context.Context, req *pb.ListRecordsSummaryRequest) (*pb.ListRecordsSummaryResponse, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent missing")
	}

	// Authentication
	parent, resultName, err := result.ParseName(req.GetParent())
	if err != nil {
		return nil, err
	}
	if err := s.auth.Check(ctx, parent, auth.ResourceRecords, auth.PermissionList); err != nil {
		return nil, err
	}

	recordsLister, err := lister.OfRecordSummary(s.recordsEnv, parent, resultName, req)
	if err != nil {
		return nil, err
	}

	aggregations, err := recordsLister.Aggregate(ctx, s.db)
	if err != nil {
		return nil, err
	}

	return &pb.ListRecordsSummaryResponse{
		Summary: aggregations,
	}, nil
}

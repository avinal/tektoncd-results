package server

import (
	"context"
	"strconv"

	"github.com/tektoncd/results/pkg/api/server/db"
	"github.com/tektoncd/results/pkg/api/server/db/errors"
	"github.com/tektoncd/results/pkg/api/server/v1alpha2/auth"
	"github.com/tektoncd/results/pkg/api/server/v1alpha2/result"
	pb "github.com/tektoncd/results/proto/v1alpha2/results_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"gorm.io/gorm"
)

func (s *Server) GetResultSummary(ctx context.Context, req *pb.GetResultRequest) (*pb.Summary, error) {
	parent, name, err := result.ParseName(req.GetName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.auth.Check(ctx, parent, auth.ResourceResults, auth.PermissionGet); err != nil {
		return nil, err
	}

	m := make(map[string]interface{})

	num, err := getNumberOfRecordsGivenResult(s.db.WithContext(ctx), parent, name)
	if err != nil {
		return nil, err
	}
	m["number"] = num

	dur, err := getAvgDurationForRecords(s.db.WithContext(ctx), parent, name)
	if err != nil {
		return nil, err
	}
	m["duration"] = dur

	data, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}

	agg := make(map[string]*pb.Aggregations)
	agg["default"] = &pb.Aggregations{Aggregations: data}

	return &pb.Summary{
		Data: agg,
	}, nil
}

func getNumberOfRecordsGivenResult(gdb *gorm.DB, parent string, name string) (string, error) {
	var count int64
	q := gdb.Model(&db.Record{}).Where(&db.Record{
		Parent:     parent,
		ResultName: name,
	}).Count(&count)
	if err := errors.Wrap(q.Error); err != nil {
		return "", err
	}
	return strconv.FormatInt(count, 10), nil
}

func getAvgDurationForRecords(gdb *gorm.DB, parent string, name string) (string, error) {
	var duration string
	q := gdb.
		Model(&db.Record{}).
		Where(&db.Record{Parent: parent, ResultName: name})
	if err := errors.Wrap(q.Error); err != nil {
		return "", err
	}
	q = q.
		Select("AVG((data->'status'->>'completionTime')::TIMESTAMP WITH TIME ZONE - (data->'status'->>'startTime')::TIMESTAMP WITH TIME ZONE)::INTERVAL").
		Scan(&duration)
	if err := errors.Wrap(q.Error); err != nil {
		return "", err
	}
	return duration, nil
}

func (s *Server) GetResultListSummary(ctx context.Context, req *pb.ResultListSummaryRequest) (*pb.Summary, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent missing")
	}

	if err := s.auth.Check(ctx, req.GetParent(), auth.ResourceResults, auth.PermissionList); err != nil {
		return nil, err
	}

	group := req.GetGroupBy()
	if group != "" && !IsValidGroup(group) {
		return nil, status.Error(codes.InvalidArgument, "group_by is not valid")
	}

	if group == "" {
		group = "default"
	}

	// execute result list query and group them

	return nil, nil

}

func IsValidGroup(group string) bool {
	// actual implementation will have more concrete groups such as status, namespace, week, month etc.
	return group == "results"
}

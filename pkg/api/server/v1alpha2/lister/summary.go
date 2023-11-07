// Copyright 2023 The Tekton Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lister

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/cel-go/cel"
	mdb "github.com/tektoncd/results/pkg/api/server/db"
	"github.com/tektoncd/results/pkg/api/server/db/errors"
	resultspb "github.com/tektoncd/results/proto/v1alpha2/results_go_proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

// requestSummary represents commonalities of ListResultsRequest and ListRecordsRequest
// objects.
type requestSummary interface {
	GetParent() string
	GetFilter() string
}

type summaryQueryBuilder interface {
	build(db *gorm.DB) (*gorm.DB, error)
}

// Aggregations is a generic utility to list, filter, sort and paginate Results and
// Records in a uniform and consistent manner.
type Aggregators struct {
	queryBuilder []queryBuilder
	group        string
}

func (a *Aggregators) buildQuery(ctx context.Context, db *gorm.DB) (*gorm.DB, error) {
	var err error
	db = db.WithContext(ctx)
	db = db.Debug()
	db = db.Model(&mdb.Record{})
	db = db.Select(a.selectFromGroup())
	for _, builder := range a.queryBuilder {
		// Add clauses for filtering.
		db, err = builder.build(db)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	db.Group(a.groupBy())
	return db, nil
}

// List lists resources applying filters, sorting elements and handling
// pagination. It returns resources in their wire form and a token to be used
// later for retrieving more pages if applicable.
func (a *Aggregators) Aggregate(ctx context.Context, db *gorm.DB) (*resultspb.Summary, error) {
	var err error
	db, err = a.buildQuery(ctx, db)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	db.Find(&results)
	if err := errors.Wrap(db.Error); err != nil {
		return nil, err
	}

	log.Printf("86********results****************%+v", results)
	return a.generateSummary(results)
}

func (Aggregators) generateSummary(results []map[string]interface{}) (*resultspb.Summary, error) {
	summary := &resultspb.Summary{
		Data: map[string]*resultspb.Aggregations{},
	}

	for i := range results {
		for k, v := range results[i] {
			if k == "created" {
				summary_key := fmt.Sprintf("%v", v)
				results[i][k] = summary_key
				data, err := structpb.NewStruct(results[i])
				if err != nil {
					return nil, err
				}
				summary.Data[summary_key] = &resultspb.Aggregations{
					Aggregations: data,
				}
			}
		}
	}
	return summary, nil
}

func (a *Aggregators) selectFromGroup() string {
	return "DATE_TRUNC('" + a.group + "', created_time) AS created, COUNT(id) AS total, data->'status'->'conditions'->0->>'status' AS status, AVG((data->'status'->>'completionTime')::TIMESTAMP WITH TIME ZONE - (data->'status'->>'startTime')::TIMESTAMP WITH TIME ZONE) AS avg_duration, MAX((data->'status'->>'completionTime')::TIMESTAMP WITH TIME ZONE - (data->'status'->>'startTime')::TIMESTAMP WITH TIME ZONE) AS max_duration"
}

func (a *Aggregators) groupBy() string {
	return "DATE_TRUNC('" + a.group + "', created_time), data->'status'->'conditions'->0->>'status'"
}

func newAggregator(env *cel.Env, fieldsToColumns map[string]string, req *resultspb.ListRecordsSummaryRequest, clauses ...equalityClause) (*Aggregators, error) {

	group := req.GetGroupBy()

	if !IsValidGroup(group) {
		return nil, status.Error(codes.InvalidArgument, "group_by is not valid: "+group)
	}

	filter := &filter{
		env:             env,
		expr:            strings.TrimSpace(req.GetFilter()),
		equalityClauses: clauses,
	}

	return &Aggregators{
		queryBuilder: []queryBuilder{
			filter,
		},
		group: group,
	}, nil
}

// OfRecordSummary creates a Lister for Record objects.
func OfRecordSummary(env *cel.Env, resultParent, resultName string, request *resultspb.ListRecordsSummaryRequest) (*Aggregators, error) {
	return newAggregator(env, recordFieldsToColumns, request, equalityClause{
		columnName: "parent",
		value:      resultParent,
	},
		equalityClause{
			columnName: "result_name",
			value:      resultName,
		})
}

func IsValidGroup(group string) bool {
	switch group {
	case "year":
		return true
	case "quarter":
		return true
	case "month":
		return true
	case "week":
		return true
	case "day":
		return true
	case "hour":
		return true
	default:
		return false
	}
}

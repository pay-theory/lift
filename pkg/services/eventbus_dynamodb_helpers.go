package services

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
)

// applyTimeRangeToKeyCondition adds time range constraints to a DynamoDB key condition
// The useGSI parameter determines whether to use published_at (GSI) or sk (main table)
func applyTimeRangeToKeyCondition(keyCondition expression.KeyConditionBuilder, query *EventQuery, useGSI bool) expression.KeyConditionBuilder {
	if useGSI {
		return applyTimeRangeToGSI(keyCondition, query)
	}
	return applyTimeRangeToMainTable(keyCondition, query)
}

// applyTimeRangeToMainTable applies time filters to the main table's composite sort key (timestamp#ulid)
func applyTimeRangeToMainTable(keyCondition expression.KeyConditionBuilder, query *EventQuery) expression.KeyConditionBuilder {
	switch {
	case query.StartTime != nil && query.EndTime != nil:
		// For range queries with composite sort keys (timestamp#ulid format)
		// We use the timestamp part for comparison with # separator
		startKey := fmt.Sprintf("%d#", query.StartTime.UnixNano())
		endKey := fmt.Sprintf("%d#", query.EndTime.UnixNano()+1) // Add 1 nanosecond for exclusive upper bound
		return keyCondition.And(
			expression.Key("sk").Between(expression.Value(startKey), expression.Value(endKey)),
		)
	case query.StartTime != nil:
		startKey := fmt.Sprintf("%d#", query.StartTime.UnixNano())
		return keyCondition.And(
			expression.Key("sk").GreaterThanEqual(expression.Value(startKey)),
		)
	case query.EndTime != nil:
		// For end time, we want everything less than endTime
		endKey := fmt.Sprintf("%d#", query.EndTime.UnixNano())
		return keyCondition.And(
			expression.Key("sk").LessThan(expression.Value(endKey)),
		)
	default:
		return keyCondition
	}
}

// applyTimeRangeToGSI applies time filters to the GSI's published_at attribute (RFC3339 format)
func applyTimeRangeToGSI(keyCondition expression.KeyConditionBuilder, query *EventQuery) expression.KeyConditionBuilder {
	switch {
	case query.StartTime != nil && query.EndTime != nil:
		// GSI uses published_at as RFC3339 strings
		startKey := query.StartTime.Format(time.RFC3339Nano)
		endKey := query.EndTime.Format(time.RFC3339Nano)
		return keyCondition.And(
			expression.Key("published_at").Between(expression.Value(startKey), expression.Value(endKey)),
		)
	case query.StartTime != nil:
		startKey := query.StartTime.Format(time.RFC3339Nano)
		return keyCondition.And(
			expression.Key("published_at").GreaterThanEqual(expression.Value(startKey)),
		)
	case query.EndTime != nil:
		endKey := query.EndTime.Format(time.RFC3339Nano)
		return keyCondition.And(
			expression.Key("published_at").LessThanEqual(expression.Value(endKey)),
		)
	default:
		return keyCondition
	}
}

// buildTagFilterExpression builds a filter expression for tags
func buildTagFilterExpression(tags []string) (expression.ConditionBuilder, bool) {
	if len(tags) == 0 {
		return expression.ConditionBuilder{}, false
	}

	var filterExpr expression.ConditionBuilder
	for i, tag := range tags {
		condition := expression.Contains(expression.Name("tags"), tag)
		if i == 0 {
			filterExpr = condition
		} else {
			filterExpr = filterExpr.And(condition)
		}
	}

	return filterExpr, true
}

// buildQueryExpression builds a complete DynamoDB query expression
func buildQueryExpression(keyCondition expression.KeyConditionBuilder, filterExpr expression.ConditionBuilder, hasFilter bool) (expression.Expression, error) {
	builder := expression.NewBuilder().WithKeyCondition(keyCondition)

	if hasFilter {
		builder = builder.WithFilter(filterExpr)
	}

	return builder.Build()
}

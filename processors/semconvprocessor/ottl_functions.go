// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvprocessor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

// ottlFunctions returns all available OTTL functions including custom ones
func ottlFunctions[K any]() map[string]ottl.Factory[K] {
	// Start with standard OTTL functions
	funcs := ottlfuncs.StandardFuncs[K]()
	
	// Add custom functions
	funcs["NormalizePath"] = normalizePathFactory[K]()
	funcs["ParseSQL"] = parseSQLFactory[K]()
	funcs["RemoveQueryParams"] = removeQueryParamsFactory[K]()
	funcs["FirstNonNil"] = firstNonNilFactory[K]()
	
	return funcs
}


// normalizePathFactory creates a NormalizePath function
func normalizePathFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("NormalizePath", &normalizePathArguments[K]{}, createNormalizePathFunction[K])
}

type normalizePathArguments[K any] struct {
	Path ottl.StringGetter[K]
}

func createNormalizePathFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*normalizePathArguments[K])
	if !ok {
		return nil, fmt.Errorf("NormalizePathFactory args must be of type *normalizePathArguments")
	}

	return normalizePath(args.Path), nil
}

func normalizePath[K any](path ottl.StringGetter[K]) ottl.ExprFunc[K] {
	// Compile regex patterns once
	uuidRe := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	numericRe := regexp.MustCompile(`/\d+(/|$)`)
	hexRe := regexp.MustCompile(`/[0-9a-fA-F]{16,}(/|$)`)
	
	return ottl.ExprFunc[K](func(ctx context.Context, tCtx K) (any, error) {
		pathStr, err := path.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		
		// Remove query parameters first
		if idx := strings.Index(pathStr, "?"); idx != -1 {
			pathStr = pathStr[:idx]
		}
		
		// Replace UUIDs with {id}
		pathStr = uuidRe.ReplaceAllString(pathStr, "{id}")
		
		// Replace hex strings (like MongoDB ObjectIds) with {id}
		pathStr = hexRe.ReplaceAllString(pathStr, "/{id}$1")
		
		// Replace numeric IDs with {id}
		pathStr = numericRe.ReplaceAllString(pathStr, "/{id}$1")
		
		return pathStr, nil
	})
}

// parseSQLFactory creates a ParseSQL function
func parseSQLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseSQL", &parseSQLArguments[K]{}, createParseSQLFunction[K])
}

type parseSQLArguments[K any] struct {
	Statement ottl.StringGetter[K]
}

func createParseSQLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*parseSQLArguments[K])
	if !ok {
		return nil, fmt.Errorf("ParseSQLFactory args must be of type *parseSQLArguments")
	}

	return parseSQL(args.Statement), nil
}

func parseSQL[K any](statement ottl.StringGetter[K]) ottl.ExprFunc[K] {
	// Compile regex patterns for SQL parsing
	selectRe := regexp.MustCompile(`(?i)^\s*SELECT\s+.*?\s+FROM\s+([^\s]+)`)
	insertRe := regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\s+(\S+)`)
	updateRe := regexp.MustCompile(`(?i)^\s*UPDATE\s+(\S+)`)
	deleteRe := regexp.MustCompile(`(?i)^\s*DELETE\s+FROM\s+(\S+)`)
	
	return ottl.ExprFunc[K](func(ctx context.Context, tCtx K) (any, error) {
		stmtStr, err := statement.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		
		// Normalize whitespace
		stmtStr = strings.TrimSpace(stmtStr)
		
		// Extract operation and table
		if matches := selectRe.FindStringSubmatch(stmtStr); len(matches) > 1 {
			table := cleanTableName(matches[1])
			return fmt.Sprintf("SELECT %s", table), nil
		}
		
		if matches := insertRe.FindStringSubmatch(stmtStr); len(matches) > 1 {
			table := cleanTableName(matches[1])
			return fmt.Sprintf("INSERT %s", table), nil
		}
		
		if matches := updateRe.FindStringSubmatch(stmtStr); len(matches) > 1 {
			table := cleanTableName(matches[1])
			return fmt.Sprintf("UPDATE %s", table), nil
		}
		
		if matches := deleteRe.FindStringSubmatch(stmtStr); len(matches) > 1 {
			table := cleanTableName(matches[1])
			return fmt.Sprintf("DELETE %s", table), nil
		}
		
		// If we can't parse it, return the first word as operation
		parts := strings.Fields(stmtStr)
		if len(parts) > 0 {
			return strings.ToUpper(parts[0]), nil
		}
		
		return "UNKNOWN", nil
	})
}

// cleanTableName removes schema prefix and quotes from table name
func cleanTableName(table string) string {
	// Remove quotes first
	table = strings.Trim(table, "`\"'[]")
	
	// Handle schema.table format - split and take the table part
	parts := strings.Split(table, ".")
	if len(parts) > 1 {
		// Get the last part (table name) and remove quotes from it too
		table = strings.Trim(parts[len(parts)-1], "`\"'[]")
	}
	
	return table
}

// removeQueryParamsFactory creates a RemoveQueryParams function
func removeQueryParamsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("RemoveQueryParams", &removeQueryParamsArguments[K]{}, createRemoveQueryParamsFunction[K])
}

type removeQueryParamsArguments[K any] struct {
	Path ottl.StringGetter[K]
}

func createRemoveQueryParamsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*removeQueryParamsArguments[K])
	if !ok {
		return nil, fmt.Errorf("RemoveQueryParamsFactory args must be of type *removeQueryParamsArguments")
	}

	return removeQueryParams(args.Path), nil
}

func removeQueryParams[K any](path ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return ottl.ExprFunc[K](func(ctx context.Context, tCtx K) (any, error) {
		pathStr, err := path.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		
		if idx := strings.Index(pathStr, "?"); idx != -1 {
			return pathStr[:idx], nil
		}
		
		return pathStr, nil
	})
}

// firstNonNilFactory creates a FirstNonNil function
func firstNonNilFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("FirstNonNil", &firstNonNilArguments[K]{}, createFirstNonNilFunction[K])
}

type firstNonNilArguments[K any] struct {
	Values []ottl.Getter[K] `ottlarg:"0"`
}

func createFirstNonNilFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*firstNonNilArguments[K])
	if !ok {
		return nil, fmt.Errorf("FirstNonNilFactory args must be of type *firstNonNilArguments")
	}

	return firstNonNil(args.Values), nil
}

func firstNonNil[K any](values []ottl.Getter[K]) ottl.ExprFunc[K] {
	return ottl.ExprFunc[K](func(ctx context.Context, tCtx K) (any, error) {
		for _, getter := range values {
			value, err := getter.Get(ctx, tCtx)
			if err != nil {
				// Continue to next value if there's an error
				continue
			}
			if value != nil {
				return value, nil
			}
		}
		// If all values are nil or errored, return nil
		return nil, nil
	})
}
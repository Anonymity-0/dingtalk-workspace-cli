// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package drive

import (
	"fmt"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

type drivePageOptions struct {
	PageAll        bool
	PageSize       int
	MaxPages       int
	MaxItems       int
	Cursor         string
	Server         string
	Tool           string
	OutputKey      string
	PageSizeParam  string
	CursorParam    string
	CollectionKeys []string
	Project        func([]any) []map[string]any
}

func collectDrivePages(rt *shortcut.RuntimeContext, base map[string]any, options drivePageOptions) (map[string]any, error) {
	if options.MaxPages <= 0 {
		options.MaxPages = 20
	}
	if options.MaxItems <= 0 {
		options.MaxItems = 500
	}
	pageLimit := 1
	if options.PageAll {
		pageLimit = options.MaxPages
	}

	items := make([]map[string]any, 0)
	seenItems := map[string]bool{}
	seenCursors := map[string]bool{}
	cursor := strings.TrimSpace(options.Cursor)
	if cursor != "" {
		seenCursors[cursor] = true
	}
	complete := false
	truncated := false
	stopReason := ""
	nextCursor := ""
	hasMore := false
	pagesRead := 0

	for page := 1; page <= pageLimit; page++ {
		remaining := options.MaxItems - len(items)
		requestPageSize := options.PageSize
		if requestPageSize > remaining {
			requestPageSize = remaining
		}
		params := cloneDriveMap(base)
		params[options.PageSizeParam] = requestPageSize
		if cursor != "" {
			params[options.CursorParam] = cursor
		}
		data, err := rt.CallMCPData(options.Server, options.Tool, params)
		if err != nil {
			return nil, drivePaginationError(options, "page_read_failed", err, page, items, cursor)
		}
		pagesRead++
		rawItems, pageState, err := requireDriveCollection(data, options.Server+"/"+options.Tool, options.CollectionKeys...)
		if err != nil {
			return nil, drivePaginationError(options, "invalid_page", err, page, items, cursor)
		}
		projected := options.Project(rawItems)
		pageItems := make([]map[string]any, 0, len(projected))
		pageSeen := map[string]bool{}
		for _, item := range projected {
			key := drivePageItemKey(item)
			if key != "" && (seenItems[key] || pageSeen[key]) {
				continue
			}
			if key != "" {
				pageSeen[key] = true
			}
			pageItems = append(pageItems, item)
		}
		if len(pageItems) > remaining {
			return nil, drivePaginationError(options, "page_size_exceeded", nil, page, items, cursor)
		}
		for _, item := range pageItems {
			if key := drivePageItemKey(item); key != "" {
				seenItems[key] = true
			}
			items = append(items, item)
		}

		pageHasMore, hasMoreKnown, next := drivePageState(pageState)
		nextCursor = strings.TrimSpace(next)
		switch {
		case hasMoreKnown:
			hasMore = pageHasMore
			complete = !pageHasMore
		case nextCursor != "":
			hasMore = true
		case len(projected) < requestPageSize:
			complete = true
		default:
			hasMore = true
			stopReason = "pagination_unproven"
		}
		if len(items) >= options.MaxItems && hasMore {
			truncated = true
			stopReason = "max_items"
		}
		if truncated {
			complete = false
			hasMore = true
		}

		if truncated || complete || !options.PageAll {
			break
		}
		if stopReason == "pagination_unproven" {
			return nil, drivePaginationError(options, stopReason, nil, page, items, cursor)
		}
		if nextCursor == "" {
			return nil, drivePaginationError(options, "missing_next_cursor", nil, page, items, cursor)
		}
		if seenCursors[nextCursor] {
			return nil, drivePaginationError(options, "stalled_cursor", nil, page, items, nextCursor)
		}
		seenCursors[nextCursor] = true
		cursor = nextCursor
		if page == pageLimit {
			truncated = true
			stopReason = "max_pages"
		}
	}

	return map[string]any{
		"contractVersion": "drive.list.v1",
		"status":          "success",
		"count":           len(items),
		options.OutputKey: items,
		"pagesRead":       pagesRead,
		"complete":        complete,
		"truncated":       truncated,
		"hasMore":         hasMore,
		"nextCursor":      nextCursor,
		"stopReason":      stopReason,
		"failures":        []map[string]any{},
	}, nil
}

func drivePaginationError(options drivePageOptions, reason string, cause error, page int, items []map[string]any, cursor string) error {
	message := fmt.Sprintf("%s 分页未完成，已在第 %d 页停止", options.Tool, page)
	if cause != nil {
		message += ": " + cause.Error()
	}
	return apperrors.NewAPI(
		message,
		apperrors.WithOperation(options.Server+"/"+options.Tool),
		apperrors.WithReason("drive_pagination_"+reason),
		apperrors.WithFailureStage("pagination"),
		apperrors.WithExecutionStarted(false),
		apperrors.WithRetryable(cause != nil),
		apperrors.WithActions("保留 nextCursor 后继续读取", "缩小查询范围后重试"),
		apperrors.WithDetails(map[string]any{
			"contractVersion": "drive.list.v1",
			"status":          "partial_success",
			"complete":        false,
			"reason":          reason,
			"page":            page,
			"nextCursor":      cursor,
			"count":           len(items),
			options.OutputKey: items,
		}),
		apperrors.WithCause(cause),
	)
}

func cloneDriveMap(source map[string]any) map[string]any {
	out := make(map[string]any, len(source)+2)
	for key, value := range source {
		out[key] = value
	}
	return out
}

func drivePageItemKey(item map[string]any) string {
	for _, key := range []string{"nodeId", "dentryId", "id", "docUrl", "url"} {
		if value, ok := item[key].(string); ok && strings.TrimSpace(value) != "" {
			return key + ":" + strings.TrimSpace(value)
		}
	}
	return ""
}

func drivePageState(data map[string]any) (bool, bool, string) {
	hasMore, known := boolField(data, "hasMore", "has_more")
	next := firstString(data, "nextCursor", "nextToken", "nextPageToken", "next_page_token")
	return hasMore, known, next
}

package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultPagedCommandPageLimit = 50
	maxPagedCommandPageLimit     = 500
	defaultPagedCommandDelayMS   = 200
)

type PagedCursorKind int

const (
	PagedCursorString PagedCursorKind = iota
	PagedCursorInt64
)

type PagedMCPCommandConfig struct {
	ServerID    string
	ToolName    string
	ItemPath    string
	CursorPath  string
	HasMorePath string
	CursorArg   string
	CursorKind  PagedCursorKind
	BuildArgs   func(*cobra.Command) (map[string]any, error)
	Fallback    func(map[string]any) error
}

type pagedCommandOptions struct {
	pageAll   bool
	pageLimit int
	maxItems  int
	delayMS   int
}

func AddPagedMCPFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("page-all", false, "自动按 nextCursor 拉取所有分页；未设置时保持单页调用")
	cmd.Flags().Int("page-limit", defaultPagedCommandPageLimit, "自动翻页最多请求页数（默认 50，范围 1-500；仅 --page-all 生效）")
	cmd.Flags().Int("max-items", 0, "自动翻页最多返回条数（默认 0 表示不限制；仅 --page-all 生效）")
	cmd.Flags().Int("page-delay", defaultPagedCommandDelayMS, "自动翻页每页之间等待毫秒数（默认 200；0 表示不等待；仅 --page-all 生效）")
}

func RunPagedMCPCommand(cmd *cobra.Command, cfg PagedMCPCommandConfig) error {
	args, err := cfg.BuildArgs(cmd)
	if err != nil {
		return err
	}
	opts, err := readPagedCommandOptions(cmd)
	if err != nil {
		return err
	}
	if !opts.pageAll {
		return cfg.Fallback(args)
	}
	if err := validatePagedConfig(cfg); err != nil {
		return err
	}
	if deps.Caller.DryRun() {
		return deps.Out.PrintJSON(map[string]any{
			"dry_run": true,
			"request": map[string]any{
				"server": cfg.ServerID,
				"name":   cfg.ToolName,
				"args":   args,
			},
			"paging": map[string]any{
				"pageAll":   true,
				"pageLimit": opts.pageLimit,
				"maxItems":  opts.maxItems,
				"pageDelay": opts.delayMS,
			},
		})
	}
	return runPagedMCPCommand(cmd, cfg, opts, args)
}

func readPagedCommandOptions(cmd *cobra.Command) (pagedCommandOptions, error) {
	pageAll, _ := cmd.Flags().GetBool("page-all")
	opts := pagedCommandOptions{pageAll: pageAll}
	if !pageAll {
		return opts, nil
	}
	opts.pageLimit, _ = cmd.Flags().GetInt("page-limit")
	if opts.pageLimit < 1 || opts.pageLimit > maxPagedCommandPageLimit {
		return opts, fmt.Errorf("--page-limit must be between 1 and 500")
	}
	opts.maxItems, _ = cmd.Flags().GetInt("max-items")
	if opts.maxItems < 0 {
		return opts, fmt.Errorf("--max-items must be greater than or equal to 0")
	}
	opts.delayMS, _ = cmd.Flags().GetInt("page-delay")
	if opts.delayMS < 0 {
		return opts, fmt.Errorf("--page-delay must be greater than or equal to 0")
	}
	return opts, nil
}

func validatePagedConfig(cfg PagedMCPCommandConfig) error {
	switch {
	case strings.TrimSpace(cfg.ServerID) == "":
		return fmt.Errorf("paged command server is required")
	case strings.TrimSpace(cfg.ToolName) == "":
		return fmt.Errorf("paged command tool is required")
	case strings.TrimSpace(cfg.ItemPath) == "":
		return fmt.Errorf("paged command item path is required")
	case strings.TrimSpace(cfg.CursorPath) == "":
		return fmt.Errorf("paged command cursor path is required")
	case strings.TrimSpace(cfg.HasMorePath) == "":
		return fmt.Errorf("paged command hasMore path is required")
	case strings.TrimSpace(cfg.CursorArg) == "":
		return fmt.Errorf("paged command cursor arg is required")
	case cfg.BuildArgs == nil || cfg.Fallback == nil:
		return fmt.Errorf("paged command callbacks are required")
	default:
		return nil
	}
}

func runPagedMCPCommand(cmd *cobra.Command, cfg PagedMCPCommandConfig, opts pagedCommandOptions, args map[string]any) error {
	var envelope map[string]any
	var allItems []any
	seenCursors := map[string]bool{}
	currentCursor := cursorValueKey(args[cfg.CursorArg], cfg.CursorKind)
	lastCursor := args[cfg.CursorArg]
	hasMore := true

	for page := 1; page <= opts.pageLimit && hasMore; page++ {
		seenCursors[currentCursor] = true
		text, err := callMCPToolReturnTextOnServer(context.Background(), cfg.ServerID, cfg.ToolName, args)
		if err != nil {
			return handlePagedCommandError(cmd, envelope, cfg, allItems, page, currentCursor, err)
		}
		parsed, items, nextCursor, more, err := parsePagedCommandPage(text, cfg)
		if err != nil {
			return handlePagedCommandError(cmd, envelope, cfg, allItems, page, currentCursor, err)
		}
		if envelope == nil {
			envelope = parsed
		}
		allItems = append(allItems, items...)
		lastCursor = nextCursor
		hasMore = more

		truncatedByItems := truncatePagedItems(&allItems, opts.maxItems)
		if truncatedByItems {
			writePagedCommandResult(envelope, cfg, allItems, pagingMetadata{
				Truncated:  true,
				HasMore:    true,
				LastCursor: lastCursor,
				Pages:      page,
				Total:      len(allItems),
			})
			return nil
		}
		if !hasMore {
			writePagedCommandResult(envelope, cfg, allItems, pagingMetadata{
				Truncated:  false,
				HasMore:    false,
				LastCursor: lastCursor,
				Pages:      page,
				Total:      len(allItems),
			})
			return nil
		}
		nextKey := cursorValueKey(nextCursor, cfg.CursorKind)
		if nextKey == "" || nextKey == currentCursor || seenCursors[nextKey] {
			err := fmt.Errorf("pagination cursor did not advance: %s", nextKey)
			return handlePagedCommandError(cmd, envelope, cfg, allItems, page+1, nextKey, err)
		}
		currentCursor = nextKey
		args[cfg.CursorArg] = normalizeCursorArg(nextCursor, cfg.CursorKind)
		if opts.delayMS > 0 {
			helperSleep(time.Duration(opts.delayMS) * time.Millisecond)
		}
	}

	writePagedCommandResult(envelope, cfg, allItems, pagingMetadata{
		Truncated:  hasMore,
		HasMore:    hasMore,
		LastCursor: lastCursor,
		Pages:      opts.pageLimit,
		Total:      len(allItems),
	})
	return nil
}

func parsePagedCommandPage(text string, cfg PagedMCPCommandConfig) (map[string]any, []any, any, bool, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, nil, nil, false, fmt.Errorf("parse paged response JSON: %w", err)
	}
	rawItems, ok := getJSONPath(parsed, cfg.ItemPath)
	if !ok {
		return nil, nil, nil, false, fmt.Errorf("paged response missing %s", cfg.ItemPath)
	}
	items, ok := rawItems.([]any)
	if !ok {
		return nil, nil, nil, false, fmt.Errorf("paged response %s must be array", cfg.ItemPath)
	}
	rawHasMore, ok := getJSONPath(parsed, cfg.HasMorePath)
	if !ok {
		return nil, nil, nil, false, fmt.Errorf("paged response missing %s", cfg.HasMorePath)
	}
	hasMore, ok := rawHasMore.(bool)
	if !ok {
		return nil, nil, nil, false, fmt.Errorf("paged response %s must be boolean", cfg.HasMorePath)
	}
	nextCursor, ok := getJSONPath(parsed, cfg.CursorPath)
	if hasMore && !ok {
		return nil, nil, nil, false, fmt.Errorf("paged response missing %s", cfg.CursorPath)
	}
	return parsed, items, nextCursor, hasMore, nil
}

type pagingMetadata struct {
	Truncated    bool
	HasMore      bool
	LastCursor   any
	Pages        int
	Total        int
	Partial      bool
	FailedPage   int
	FailedCursor string
	PagesFetched int
	ItemsFetched int
	Error        string
}

func handlePagedCommandError(cmd *cobra.Command, envelope map[string]any, cfg PagedMCPCommandConfig, items []any, failedPage int, failedCursor string, err error) error {
	if envelope == nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "pagination stopped at page %d: %v\n", failedPage, err)
	writePagedCommandResult(envelope, cfg, items, pagingMetadata{
		Truncated:    true,
		HasMore:      true,
		LastCursor:   failedCursor,
		Pages:        failedPage - 1,
		Total:        len(items),
		Partial:      true,
		FailedPage:   failedPage,
		FailedCursor: failedCursor,
		PagesFetched: failedPage - 1,
		ItemsFetched: len(items),
		Error:        err.Error(),
	})
	return err
}

func writePagedCommandResult(envelope map[string]any, cfg PagedMCPCommandConfig, items []any, meta pagingMetadata) {
	_ = setJSONPath(envelope, cfg.ItemPath, items)
	paging := map[string]any{
		"truncated":  meta.Truncated,
		"hasMore":    meta.HasMore,
		"lastCursor": meta.LastCursor,
		"pages":      meta.Pages,
		"total":      meta.Total,
	}
	if meta.Partial {
		paging["partial"] = true
		paging["failedPage"] = meta.FailedPage
		paging["failedCursor"] = meta.FailedCursor
		paging["pagesFetched"] = meta.PagesFetched
		paging["itemsFetched"] = meta.ItemsFetched
		paging["error"] = meta.Error
	}
	envelope["paging"] = paging
	_ = deps.Out.PrintJSON(envelope)
}

func truncatePagedItems(items *[]any, maxItems int) bool {
	if maxItems <= 0 || len(*items) <= maxItems {
		return false
	}
	*items = (*items)[:maxItems]
	return true
}

func getJSONPath(root map[string]any, path string) (any, bool) {
	var current any = root
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setJSONPath(root map[string]any, path string, value any) bool {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
	return true
}

func cursorValueKey(value any, kind PagedCursorKind) string {
	switch kind {
	case PagedCursorInt64:
		switch v := value.(type) {
		case int64:
			return strconv.FormatInt(v, 10)
		case int:
			return strconv.Itoa(v)
		case float64:
			return strconv.FormatInt(int64(v), 10)
		case string:
			return strings.TrimSpace(v)
		default:
			return ""
		}
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func normalizeCursorArg(value any, kind PagedCursorKind) any {
	if kind != PagedCursorInt64 {
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return int64(0)
}

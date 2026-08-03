// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Package targetresolver provides deterministic natural-target resolution for
// shortcuts. It owns extraction, stable-ID de-duplication, exact-match
// preference, ambiguity handling, and the machine-readable resolution error
// envelope shared by send/read/search/create/event facades.
package targetresolver

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

// Status is the stable resolution outcome used in successful plans and errors.
type Status string

const (
	StatusResolved  Status = "resolved"
	StatusAmbiguous Status = "ambiguous"
	StatusNotFound  Status = "not_found"
)

// IdentityRequirement filters contacts to identities accepted by the
// downstream interface.
type IdentityRequirement string

const (
	IdentityAny            IdentityRequirement = "any"
	IdentityUserID         IdentityRequirement = "user_id"
	IdentityOpenDingTalkID IdentityRequirement = "open_dingtalk_id"
)

// Reader is the minimal read-only transport needed by natural target
// resolution. Shortcut RuntimeContext implements it, and native facades (for
// example event +listen-im) can provide an adapter without duplicating
// extraction or ambiguity rules.
type Reader interface {
	CallMCPData(product, tool string, params map[string]any) (map[string]any, error)
}

// User is the public, credential-free identity returned by contact resolution.
type User struct {
	UserID         string `json:"userId,omitempty"`
	OpenDingTalkID string `json:"openDingTalkId,omitempty"`
	Name           string `json:"name,omitempty"`
}

// UserResolution is the typed successful user-resolution result.
type UserResolution struct {
	Status     Status `json:"status"`
	EntityType string `json:"entityType"`
	Query      string `json:"query"`
	MatchType  string `json:"matchType"`
	Selected   User   `json:"selected"`
	Profile    string `json:"profile,omitempty"`
}

// Chat is the public identity returned by group resolution.
type Chat struct {
	OpenConversationID string `json:"openConversationId"`
	Name               string `json:"name,omitempty"`
}

// ChatResolution is the typed successful group-resolution result.
type ChatResolution struct {
	Status     Status `json:"status"`
	EntityType string `json:"entityType"`
	Query      string `json:"query"`
	MatchType  string `json:"matchType"`
	Selected   Chat   `json:"selected"`
	Profile    string `json:"profile,omitempty"`
}

// ResolveUser searches the current profile's directory and resolves query to
// exactly one identity accepted by the downstream interface.
func ResolveUser(rt Reader, query string, requirement IdentityRequirement) (UserResolution, error) {
	query = strings.TrimSpace(query)
	data, err := rt.CallMCPData("contact", "search_contact_by_key_word", map[string]any{
		"keyword": query,
	})
	if err != nil {
		return UserResolution{}, err
	}
	users := filterUsersByIdentity(dedupeUsers(ExtractUsers(data)), requirement)
	selected, matchType := selectUsers(users, query)
	if len(selected) == 0 {
		return UserResolution{}, newResolutionError(StatusNotFound, "user", query, users)
	}
	if len(selected) > 1 {
		return UserResolution{}, newResolutionError(StatusAmbiguous, "user", query, selected)
	}
	return UserResolution{
		Status:     StatusResolved,
		EntityType: "user",
		Query:      query,
		MatchType:  matchType,
		Selected:   selected[0],
		Profile:    auth.RuntimeProfile(),
	}, nil
}

// ResolveChat searches groups and resolves query to exactly one stable
// openConversationId. Exact names win over substring matches, but multiple
// exact names remain ambiguous.
func ResolveChat(rt Reader, query string) (ChatResolution, error) {
	query = strings.TrimSpace(query)
	data, err := rt.CallMCPData("im", "search_groups", map[string]any{
		"keyword": query,
		"limit":   10,
		"cursor":  "0",
	})
	if err != nil {
		return ChatResolution{}, err
	}
	chats := dedupeChats(ExtractChats(data))
	selected, matchType := preferExactChats(chats, query)
	if len(selected) == 0 {
		return ChatResolution{}, newResolutionError(StatusNotFound, "chat", query, chats)
	}
	if len(selected) > 1 {
		return ChatResolution{}, newResolutionError(StatusAmbiguous, "chat", query, selected)
	}
	return ChatResolution{
		Status:     StatusResolved,
		EntityType: "chat",
		Query:      query,
		MatchType:  matchType,
		Selected:   selected[0],
		Profile:    auth.RuntimeProfile(),
	}, nil
}

// ResolveUsers resolves every query before returning. Resolution failures are
// collected into one typed envelope; upstream/auth failures still stop
// immediately. Successful identities are de-duplicated by stable ID.
func ResolveUsers(
	rt Reader,
	queries []string,
	requirement IdentityRequirement,
) ([]UserResolution, error) {
	results := make([]UserResolution, 0, len(queries))
	failures := make([]map[string]any, 0)
	for _, query := range uniqueQueries(queries) {
		resolved, err := ResolveUser(rt, query, requirement)
		if err != nil {
			if details, ok := resolutionDetails(err); ok {
				failures = append(failures, details)
				continue
			}
			return nil, err
		}
		results = append(results, resolved)
	}
	if len(failures) > 0 {
		return nil, batchResolutionError("user", failures)
	}
	seenUserIDs := map[string]bool{}
	seenOpenIDs := map[string]bool{}
	deduped := make([]UserResolution, 0, len(results))
	for _, result := range results {
		user := result.Selected
		if (user.UserID != "" && seenUserIDs[user.UserID]) ||
			(user.OpenDingTalkID != "" && seenOpenIDs[user.OpenDingTalkID]) {
			continue
		}
		seenUserIDs[user.UserID] = user.UserID != ""
		seenOpenIDs[user.OpenDingTalkID] = user.OpenDingTalkID != ""
		deduped = append(deduped, result)
	}
	return deduped, nil
}

// ResolveChats is the batch equivalent of ResolveChat and returns only after
// every query has been preflighted.
func ResolveChats(rt Reader, queries []string) ([]ChatResolution, error) {
	results := make([]ChatResolution, 0, len(queries))
	failures := make([]map[string]any, 0)
	for _, query := range uniqueQueries(queries) {
		resolved, err := ResolveChat(rt, query)
		if err != nil {
			if details, ok := resolutionDetails(err); ok {
				failures = append(failures, details)
				continue
			}
			return nil, err
		}
		results = append(results, resolved)
	}
	seen := map[string]bool{}
	deduped := make([]ChatResolution, 0, len(results))
	for _, result := range results {
		if seen[result.Selected.OpenConversationID] {
			continue
		}
		seen[result.Selected.OpenConversationID] = true
		deduped = append(deduped, result)
	}
	if len(failures) > 0 {
		return nil, batchResolutionError("chat", failures)
	}
	return deduped, nil
}

// ExtractUsers accepts the contact search response shapes used by the current
// directory MCP and keeps external contacts that only expose openDingTalkId.
func ExtractUsers(data map[string]any) []User {
	raw := firstList(data, "result", "items", "users", "list")
	if len(raw) == 0 {
		return nil
	}
	users := make([]User, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		userID := stringValue(row, "userId", "orgUserId")
		openID := stringValue(row, "openDingTalkId", "openDingtalkId")
		if userID == "" && openID == "" {
			continue
		}
		users = append(users, User{
			UserID:         userID,
			OpenDingTalkID: openID,
			Name: stringValue(row,
				"name", "nick", "showName", "flowerName", "staffName", "userName"),
		})
	}
	return users
}

// ExtractChats accepts both bare and wrapped group search result lists.
func ExtractChats(data map[string]any) []Chat {
	raw := firstList(data, "result", "items", "groups", "list")
	if len(raw) == 0 {
		return nil
	}
	chats := make([]Chat, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := stringValue(row, "openConversationId", "openconversationId", "id")
		if id == "" {
			continue
		}
		chats = append(chats, Chat{
			OpenConversationID: id,
			Name:               stringValue(row, "title", "name", "conversationName"),
		})
	}
	return chats
}

// UsersWithUserID preserves the legacy smart-shortcut helper contract.
func UsersWithUserID(users []User) []User {
	return filterUsersByIdentity(users, IdentityUserID)
}

// PreferExactChats exposes the deterministic selection helper to legacy smart
// shortcuts while they migrate to ResolveChat.
func PreferExactChats(chats []Chat, query string) []Chat {
	selected, _ := preferExactChats(dedupeChats(chats), query)
	return selected
}

// UserLabels renders credential-free disambiguation labels.
func UserLabels(users []User) []string {
	labels := make([]string, 0, len(users))
	for _, user := range users {
		id := user.UserID
		if id == "" {
			id = user.OpenDingTalkID
		}
		labels = append(labels, fmt.Sprintf("%s(%s)", user.Name, id))
	}
	return labels
}

// ChatLabels renders group disambiguation labels.
func ChatLabels(chats []Chat) []string {
	labels := make([]string, 0, len(chats))
	for _, chat := range chats {
		name := chat.Name
		if name == "" {
			name = "(未命名群)"
		}
		labels = append(labels, fmt.Sprintf("%s(%s)", name, chat.OpenConversationID))
	}
	return labels
}

func selectUsers(users []User, query string) ([]User, string) {
	if len(users) != 1 {
		return users, "ambiguous"
	}
	if strings.EqualFold(strings.TrimSpace(users[0].Name), strings.TrimSpace(query)) {
		return users, "exact"
	}
	return users, "unique"
}

func preferExactChats(chats []Chat, query string) ([]Chat, string) {
	exact := make([]Chat, 0, len(chats))
	for _, chat := range chats {
		if strings.EqualFold(strings.TrimSpace(chat.Name), strings.TrimSpace(query)) {
			exact = append(exact, chat)
		}
	}
	if len(exact) > 0 {
		return exact, "exact"
	}
	return chats, "unique"
}

func filterUsersByIdentity(users []User, requirement IdentityRequirement) []User {
	filtered := make([]User, 0, len(users))
	for _, user := range users {
		switch requirement {
		case IdentityUserID:
			if user.UserID == "" {
				continue
			}
		case IdentityOpenDingTalkID:
			if user.OpenDingTalkID == "" {
				continue
			}
		}
		filtered = append(filtered, user)
	}
	return filtered
}

func dedupeUsers(users []User) []User {
	result := make([]User, 0, len(users))
	seenUserIDs := map[string]bool{}
	seenOpenIDs := map[string]bool{}
	for _, user := range users {
		if (user.UserID != "" && seenUserIDs[user.UserID]) ||
			(user.OpenDingTalkID != "" && seenOpenIDs[user.OpenDingTalkID]) {
			continue
		}
		if user.UserID != "" {
			seenUserIDs[user.UserID] = true
		}
		if user.OpenDingTalkID != "" {
			seenOpenIDs[user.OpenDingTalkID] = true
		}
		result = append(result, user)
	}
	return result
}

func dedupeChats(chats []Chat) []Chat {
	result := make([]Chat, 0, len(chats))
	seen := map[string]bool{}
	for _, chat := range chats {
		if seen[chat.OpenConversationID] {
			continue
		}
		seen[chat.OpenConversationID] = true
		result = append(result, chat)
	}
	return result
}

func newResolutionError(status Status, entityType, query string, candidates any) error {
	details := map[string]any{
		"type":       "resolution",
		"subtype":    status,
		"entityType": entityType,
		"query":      query,
		"candidates": candidates,
	}
	if profile := auth.RuntimeProfile(); profile != "" {
		details["profile"] = profile
	}
	if status == StatusNotFound {
		return apperrors.NewValidation(
			fmt.Sprintf("没有找到与 %q 唯一匹配且可用于当前操作的%s", query, entityLabel(entityType)),
			apperrors.WithReason("resolution_not_found"),
			apperrors.WithRetryable(false),
			apperrors.WithHint("请提供更完整的名称或直接传稳定 ID"),
			apperrors.WithDetails(details),
		)
	}
	return apperrors.NewValidation(
		fmt.Sprintf("%q 匹配到多个%s：%s；请提供更精确的名称或直接传稳定 ID", query, entityLabel(entityType), candidateLabels(candidates)),
		apperrors.WithReason("resolution_ambiguous"),
		apperrors.WithRetryable(false),
		apperrors.WithHint("禁止默认选择第一个候选"),
		apperrors.WithDetails(details),
	)
}

func batchResolutionError(entityType string, failures []map[string]any) error {
	return apperrors.NewValidation(
		fmt.Sprintf("%d 个%s目标未能唯一解析；已停止后续操作", len(failures), entityLabel(entityType)),
		apperrors.WithReason("resolution_batch_failed"),
		apperrors.WithRetryable(false),
		apperrors.WithHint("请逐项消歧或直接传稳定 ID"),
		apperrors.WithDetails(map[string]any{
			"type":       "resolution",
			"subtype":    "batch_failed",
			"entityType": entityType,
			"failures":   failures,
		}),
	)
}

func resolutionDetails(err error) (map[string]any, bool) {
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.Details["type"] != "resolution" {
		return nil, false
	}
	return typed.Details, true
}

func entityLabel(entityType string) string {
	if entityType == "chat" {
		return "群聊"
	}
	return "用户"
}

func candidateLabels(candidates any) string {
	switch values := candidates.(type) {
	case []User:
		return strings.Join(UserLabels(values), "、")
	case []Chat:
		return strings.Join(ChatLabels(values), "、")
	default:
		return "候选"
	}
}

func firstList(data map[string]any, keys ...string) []any {
	if data == nil {
		return nil
	}
	scopes := []map[string]any{data}
	for _, wrapper := range []string{"result", "data"} {
		if nested, ok := data[wrapper].(map[string]any); ok {
			scopes = append(scopes, nested)
		}
	}
	for _, scope := range scopes {
		for _, key := range keys {
			if raw, ok := scope[key].([]any); ok {
				return raw
			}
		}
	}
	return nil
}

func stringValue(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueQueries(queries []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(queries))
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" || seen[query] {
			continue
		}
		seen[query] = true
		result = append(result, query)
	}
	return result
}

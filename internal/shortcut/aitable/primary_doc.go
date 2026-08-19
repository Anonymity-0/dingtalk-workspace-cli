// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

import (
	"fmt"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func executeRecordPrimaryDocGet(rt *shortcut.RuntimeContext) error {
	baseID, tableID, requestedRecordID := rt.Str("base-id"), rt.Str("table-id"), rt.Str("record-id")
	fieldsData, err := rt.CallMCPData(serverMain, "get_fields", map[string]any{"baseId": baseID, "tableId": tableID})
	if err != nil {
		return err
	}
	fields, found := findNamedObjectList(fieldsData, "fields", "fieldList")
	if !found {
		return apperrors.NewAPI("get_fields response is missing the fields collection",
			apperrors.WithOperation("aitable/get_fields"), apperrors.WithReason("target_invalid_response"),
			apperrors.WithFailureStage("response_validation"), apperrors.WithExecutionStarted(false))
	}
	primaryFieldIDs := make([]string, 0, 1)
	for index, field := range fields {
		fieldType := stringValue(field, "type", "fieldType")
		if !strings.EqualFold(fieldType, "primaryDoc") {
			continue
		}
		fieldID := stringValue(field, "fieldId", "id")
		if fieldID == "" {
			return apperrors.NewAPI(fmt.Sprintf("primaryDoc field %d is missing fieldId", index),
				apperrors.WithOperation("aitable/get_fields"), apperrors.WithReason("target_invalid_response"),
				apperrors.WithFailureStage("response_validation"), apperrors.WithExecutionStarted(false))
		}
		primaryFieldIDs = append(primaryFieldIDs, fieldID)
	}

	recordParams := map[string]any{
		"baseId": baseID, "tableId": tableID, "recordIds": []string{requestedRecordID},
	}
	if len(primaryFieldIDs) > 0 {
		recordParams["fieldIds"] = primaryFieldIDs
	}
	window, err := queryRecordWindow(rt, recordParams, 1)
	if err != nil {
		return err
	}
	records := window.Records
	if len(records) == 0 {
		return apperrors.NewAPI(fmt.Sprintf("record %s was not found", requestedRecordID),
			apperrors.WithOperation("aitable/query_records"), apperrors.WithReason("RESOURCE_NOT_FOUND"),
			apperrors.WithFailureStage("target_resolution"), apperrors.WithExecutionStarted(false))
	}
	if len(records) != 1 || recordID(records[0]) != requestedRecordID {
		return apperrors.NewAPI("record preflight did not return the exact requested record",
			apperrors.WithOperation("aitable/query_records"), apperrors.WithReason("target_invalid_response"),
			apperrors.WithFailureStage("response_validation"), apperrors.WithExecutionStarted(false))
	}
	if len(primaryFieldIDs) == 0 {
		return rt.Output(map[string]any{
			"status": "no_primary_doc_field", "exists": false, "recordId": requestedRecordID, "nodeId": nil,
		})
	}
	data, err := rt.CallMCPData(serverHelper, "get_primary_doc", map[string]any{
		"baseId": baseID, "tableId": tableID, "recordId": requestedRecordID,
	})
	if err != nil {
		if knownPrimaryDocUnassociatedError(err) {
			return rt.Output(map[string]any{
				"status": "unassociated", "exists": false, "recordId": requestedRecordID, "fieldIds": primaryFieldIDs, "nodeId": nil,
			})
		}
		return err
	}
	nodeID := findStringByKeys(data, "nodeId", "dentryUuid")
	if nodeID == "" {
		if knownPrimaryDocUnassociatedData(data) {
			return rt.Output(map[string]any{
				"status": "unassociated", "exists": false, "recordId": requestedRecordID, "fieldIds": primaryFieldIDs, "nodeId": nil,
			})
		}
		return apperrors.NewAPI("get_primary_doc response is missing nodeId",
			apperrors.WithOperation("aitable-helper/get_primary_doc"), apperrors.WithReason("target_invalid_response"),
			apperrors.WithFailureStage("response_validation"), apperrors.WithExecutionStarted(false))
	}
	return rt.Output(map[string]any{
		"status": "associated", "exists": true, "recordId": requestedRecordID, "fieldIds": primaryFieldIDs, "nodeId": nodeID,
	})
}

func knownPrimaryDocUnassociatedError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "no record") || strings.Contains(message, "unassociated")
}

func knownPrimaryDocUnassociatedData(value any) bool {
	var walk func(any) bool
	walk = func(current any) bool {
		switch typed := current.(type) {
		case map[string]any:
			if exists, ok := typed["exists"].(bool); ok && !exists {
				return true
			}
			for _, key := range []string{"nodeId", "dentryUuid"} {
				if nodeID, exists := typed[key]; exists && (nodeID == nil || strings.TrimSpace(fmt.Sprint(nodeID)) == "") {
					return true
				}
			}
			if status, ok := typed["status"].(string); ok {
				status = strings.ToLower(strings.TrimSpace(status))
				if status == "unassociated" || status == "no_record" {
					return true
				}
			}
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}

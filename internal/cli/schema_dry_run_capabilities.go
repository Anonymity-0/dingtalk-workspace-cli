// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// dryRunCapabilityGroup is a reviewed positive capability declaration. An
// absent canonical deliberately publishes no dry_run field.
type dryRunCapabilityGroup struct {
	PreviewKind    string
	CanonicalPaths []string
}

// reviewedDryRunCapabilityGroups contains only command-owned preview paths
// for tools WITHOUT a Contract final declaration. Declared tools publish
// their dry_run capability from cmdcore.SchemaDecl (reviewed code) and are
// merged into the reviewed set at assembly time — no manual list entry.
// Inheriting the root --dry-run flag or reaching the generic EchoRunner is not
// evidence of a stable capability and must never add a command to this list.
// CI executes each selected example and compares the observed preview kind to
// this reviewed declaration.
var reviewedDryRunCapabilityGroups = []dryRunCapabilityGroup{
	{PreviewKind: DryRunPreviewRequest, CanonicalPaths: []string{
		"event.stop",
	}},
	{PreviewKind: DryRunPreviewPlan, CanonicalPaths: []string{
		"chat.download_media",
		"doc.download_file",
		"doc.import_get",
		"doc.media_insert",
		"doc.query_export_job",
		"doc.upload",
		"drive.download_file",
		"drive.upload",
		"markdown.create",
		"markdown.fetch",
		"markdown.overwrite",
		"markdown.patch",
		"sheet.filter_view_get_criteria",
		"sheet.filter_view_info",
		"sheet.filter_view_list_criteria",
		"sheet.media_upload",
		"sheet.submit_export_job",
		"sheet.write_image",
		"todo.add_todo_attachment",
	}},
}

var reviewedDryRunCapabilitiesLazy struct {
	once        sync.Once
	byCanonical map[string]DryRunSpec
	err         error
}

// declaredDryRunCapabilities indexes dry_run capabilities sourced from
// Contract final declarations (canonical → spec). Populated by
// BindEffectiveCommandRegistry at command-tree bind time — every process
// that resolves the tree gets the reviewed set, not only processes that run
// Schema assembly. A declaration in reviewed code is itself the reviewed
// capability, so no manual allowlist entry is required.
var declaredDryRunCapabilities sync.Map // string → DryRunSpec

// recordDeclaredDryRunCapability registers one Contract-declared dry_run
// capability. Conflicting re-declaration of the same canonical is a
// programming error surfaced at the next delivery gate read.
func recordDeclaredDryRunCapability(canonical string, spec DryRunSpec) {
	canonical = strings.TrimSpace(canonical)
	if canonical == "" {
		return
	}
	declaredDryRunCapabilities.Store(canonical, spec)
}

// clearDeclaredDryRunCapabilitiesForTest resets the declared index (tests only).
func clearDeclaredDryRunCapabilitiesForTest() {
	declaredDryRunCapabilities.Range(func(key, _ any) bool {
		declaredDryRunCapabilities.Delete(key)
		return true
	})
}

func loadManualDryRunCapabilities() (map[string]DryRunSpec, error) {
	reviewedDryRunCapabilitiesLazy.once.Do(func() {
		byCanonical := make(map[string]DryRunSpec)
		for _, group := range reviewedDryRunCapabilityGroups {
			spec := DryRunSpec{PreviewKind: group.PreviewKind}
			if err := spec.Validate("<reviewed-dry-run-registry>"); err != nil {
				reviewedDryRunCapabilitiesLazy.err = err
				return
			}
			previous := ""
			for _, raw := range group.CanonicalPaths {
				canonical := strings.TrimSpace(raw)
				if canonical == "" {
					reviewedDryRunCapabilitiesLazy.err = fmt.Errorf("reviewed dry-run capability has empty canonical path")
					return
				}
				if previous != "" && canonical <= previous {
					reviewedDryRunCapabilitiesLazy.err = fmt.Errorf("reviewed dry-run capability paths for %s are not strictly sorted at %q", group.PreviewKind, canonical)
					return
				}
				previous = canonical
				if _, duplicate := byCanonical[canonical]; duplicate {
					reviewedDryRunCapabilitiesLazy.err = fmt.Errorf("duplicate reviewed dry-run capability %s", canonical)
					return
				}
				byCanonical[canonical] = spec
			}
		}
		reviewedDryRunCapabilitiesLazy.byCanonical = byCanonical
	})
	if reviewedDryRunCapabilitiesLazy.err != nil {
		return nil, reviewedDryRunCapabilitiesLazy.err
	}
	out := make(map[string]DryRunSpec, len(reviewedDryRunCapabilitiesLazy.byCanonical))
	for canonical, spec := range reviewedDryRunCapabilitiesLazy.byCanonical {
		out[canonical] = spec
	}
	return out, nil
}

func loadReviewedDryRunCapabilities() (map[string]DryRunSpec, error) {
	out, err := loadManualDryRunCapabilities()
	if err != nil {
		return nil, err
	}
	var mergeErr error
	declaredDryRunCapabilities.Range(func(key, value any) bool {
		canonical, ok := key.(string)
		if !ok {
			mergeErr = fmt.Errorf("declared dry-run capability has non-string key %v", key)
			return false
		}
		spec, ok := value.(DryRunSpec)
		if !ok {
			mergeErr = fmt.Errorf("declared dry-run capability %s has non-DryRunSpec value", canonical)
			return false
		}
		if manual, exists := out[canonical]; exists && manual != spec {
			mergeErr = fmt.Errorf("dry-run capability %s declared as %#v conflicts with manual reviewed entry %#v", canonical, spec, manual)
			return false
		}
		out[canonical] = spec
		return true
	})
	if mergeErr != nil {
		return nil, mergeErr
	}
	return out, nil
}

// ReviewedDryRunCapabilities returns a defensive copy of the positive,
// reviewed capability registry for delivery gates.
func ReviewedDryRunCapabilities() (map[string]DryRunSpec, error) {
	return loadReviewedDryRunCapabilities()
}

func reviewedDryRunCapability(canonical string) (*DryRunSpec, error) {
	capabilities, err := loadReviewedDryRunCapabilities()
	if err != nil {
		return nil, err
	}
	spec, ok := capabilities[strings.TrimSpace(canonical)]
	if !ok {
		return nil, nil
	}
	return &spec, nil
}

// ValidateReviewedDryRunCapabilityDelivery proves that every positive source
// entry reaches the final typed registry and no serializer invents one. It
// deliberately imposes no minimum capability count or all-command coverage.
func ValidateReviewedDryRunCapabilityDelivery(registry SchemaRegistry) error {
	expected, err := loadReviewedDryRunCapabilities()
	if err != nil {
		return err
	}
	actual := make(map[string]DryRunSpec)
	for _, product := range registry.Products {
		for _, tool := range product.Tools {
			if tool.DryRun != nil {
				actual[tool.Identity.CanonicalPath] = *tool.DryRun
			}
		}
	}
	var problems []string
	for canonical, want := range expected {
		got, ok := actual[canonical]
		if !ok {
			problems = append(problems, fmt.Sprintf("reviewed dry-run capability %s is missing from final Schema", canonical))
			continue
		}
		if got != want {
			problems = append(problems, fmt.Sprintf("Schema dry-run capability %s = %#v, want %#v", canonical, got, want))
		}
	}
	for canonical := range actual {
		if _, ok := expected[canonical]; !ok {
			problems = append(problems, fmt.Sprintf("Schema tool %s publishes an unreviewed dry-run capability", canonical))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

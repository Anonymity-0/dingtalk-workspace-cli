// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"sort"
	"strings"
	"testing"
)

// schemaParameterBindingsPhase2MigratedProducts are product prefixes whose
// active bindings were retired to leaf ParamDecl.Property in Track 1 Phase 2.
// The embed path remains for mapping_exclusions / removals audit only.
var schemaParameterBindingsPhase2MigratedProducts = []string{
	"aisearch",
	"aitable",
	"attendance",
	"calendar",
	"chat",
	"contact",
	"dev",
	"devdoc",
	"ding",
	"doc",
	"drive",
	"mail",
	"minutes",
	"oa",
	"report",
	"sheet",
	"todo",
	"wiki",
}

func TestSchemaParameterBindingsPhase2MigratedProductsHaveNoActiveRows(t *testing.T) {
	snapshot, err := loadSchemaParameterBindingSnapshot()
	if err != nil {
		t.Fatalf("loadSchemaParameterBindingSnapshot() error = %v", err)
	}
	for _, product := range schemaParameterBindingsPhase2MigratedProducts {
		prefix := product + "."
		for canonical := range snapshot.Bindings {
			if strings.HasPrefix(canonical, prefix) {
				t.Errorf("migrated product %q still has active binding canonical %q", product, canonical)
			}
		}
	}
}

func TestSchemaParameterBindingsPhase2RemainingInventory(t *testing.T) {
	snapshot, err := loadSchemaParameterBindingSnapshot()
	if err != nil {
		t.Fatalf("loadSchemaParameterBindingSnapshot() error = %v", err)
	}
	byProduct := map[string]int{}
	total := 0
	for canonical, flags := range snapshot.Bindings {
		product, _, _ := strings.Cut(canonical, ".")
		n := len(flags)
		byProduct[product] += n
		total += n
	}
	if total != 0 {
		products := make([]string, 0, len(byProduct))
		for product := range byProduct {
			products = append(products, product)
		}
		sort.Slice(products, func(i, j int) bool {
			if byProduct[products[i]] != byProduct[products[j]] {
				return byProduct[products[i]] > byProduct[products[j]]
			}
			return products[i] < products[j]
		})
		t.Fatalf("remaining active binding tuples = %d (%v), want 0 after full ParamDecl.Property retirement", total, products)
	}
	if len(snapshot.MappingExclusions) == 0 {
		t.Fatal("mapping_exclusions ledger is empty; exclusion semantics must remain reviewed in schema_parameter_bindings.json")
	}
	t.Logf("Phase 2 complete: 0 active tuples; %d mapping_exclusions; %d removals", len(snapshot.MappingExclusions), len(snapshot.Removals))
}

func TestSchemaParameterBindingsPhase2ParamDeclPropertyOutranksBinding(t *testing.T) {
	binding := runtimeSchemaStringCandidate("fromJSON", "versioned_parameter_binding")
	paramDecl := runtimeSchemaStringCandidateAtRank(
		"fromParamDecl",
		"native_annotation",
		runtimeSchemaRankParamDeclProperty,
		runtimeSchemaPrecedenceNativeAnnotation,
	)
	winner, err := resolveRuntimeSchemaCandidate("property", binding, paramDecl)
	if err != nil {
		t.Fatalf("resolveRuntimeSchemaCandidate() error = %v", err)
	}
	if winner.Source != "native_annotation" || winner.Value != "fromParamDecl" {
		t.Fatalf("ParamDecl.Property dual-read winner = %#v, want native fromParamDecl", winner)
	}
}

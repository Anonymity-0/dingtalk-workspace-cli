// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agentmetadata

import (
	"fmt"
	"strings"
)

// parseHintSources formerly loaded schema_hints/ HintFiles. That directory is
// fully retired: ProductDecl / ContractFinal own routing and leaf facts.
// An empty HintsDir is a no-op; any non-empty value fails closed.
func parseHintSources(_ *File, _ []sourceFile, opts Options, _ *Stats, _ sourceTracker) (usedSelection bool, err error) {
	if strings.TrimSpace(opts.HintsDir) == "" {
		return false, nil
	}
	return false, fmt.Errorf("schema_hints/ is retired; clear HintsDir and declare ProductDecl/ContractFinal instead (got %q)", opts.HintsDir)
}

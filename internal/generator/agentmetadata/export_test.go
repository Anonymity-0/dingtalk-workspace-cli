// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package agentmetadata

// ValidateSelectionAuthoringContractsForTest exposes the selection authoring
// gate to external tests that must import app without polluting this package.
func ValidateSelectionAuthoringContractsForTest(opts Options) error {
	return validateSelectionAuthoringContracts(opts)
}

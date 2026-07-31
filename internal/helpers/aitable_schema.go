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

package helpers

import "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"

// aitable_schema.go holds shared Safety / Interface factories for aitable's
// DeclareLeafMetadata declarations (metadata-only mode). Selection prose and
// per-command payloads live in aitable_schema_decls_generated.go.

func aitableSafetyRead() cli.SafetySpec {
	return cli.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	}
}

func aitableSafetyWrite() cli.SafetySpec {
	return cli.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "not_required", Idempotency: "unknown",
	}
}

func aitableSafetyWriteConfirm() cli.SafetySpec {
	return cli.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "user_required", Idempotency: "unknown",
	}
}

func aitableSafetyDestructive() cli.SafetySpec {
	return cli.SafetySpec{
		Effect: "destructive", Risk: "high",
		Confirmation: "user_required", Idempotency: "unknown",
	}
}

func aitableMCPInterface(rpc string) *LeafInterfaceDecl {
	return &LeafInterfaceDecl{
		Mode: "mcp", Availability: "available",
		ProductID: "aitable", RPCName: rpc,
	}
}

func aitableHelperMCPInterface(rpc string) *LeafInterfaceDecl {
	return &LeafInterfaceDecl{
		Mode: "mcp", Availability: "available",
		ProductID: "aitable-helper", RPCName: rpc,
	}
}

func aitableCompositeInterface(reason string) *LeafInterfaceDecl {
	return &LeafInterfaceDecl{
		Mode: "composite", Availability: "available", Reason: reason,
	}
}

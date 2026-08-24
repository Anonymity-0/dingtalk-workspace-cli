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

package app

import (
	"os"
	"testing"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
)

func TestNewCommandRunnerDoesNotBackfillCredentialEnvironment(t *testing.T) {
	t.Setenv(authpkg.EnvClientID, "")
	t.Setenv(authpkg.EnvClientSecret, "")
	authpkg.SetClientID("persisted-looking-id")
	authpkg.SetClientSecret("")
	t.Cleanup(func() {
		authpkg.SetClientID("")
		authpkg.SetClientSecret("")
	})

	_ = newCommandRunnerWithFlags(&GlobalFlags{})
	if os.Getenv(authpkg.EnvClientID) != "" || os.Getenv(authpkg.EnvClientSecret) != "" {
		t.Fatalf("runner mutated credential env: id=%q secret_set=%t", os.Getenv(authpkg.EnvClientID), os.Getenv(authpkg.EnvClientSecret) != "")
	}
}

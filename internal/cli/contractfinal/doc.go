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

// Package contractfinal is a thin delivery-facing re-export of
// internal/corecmd/contractfinal.
//
// Ownership of the Cobra-keyed ContractFinal store and Register seam lives
// under the command framework. Production product code should prefer
// cli.RegisterRuntimeContractFinal; framework code calls
// corecmd/contractfinal.RegisterRuntimeContractFinal directly.
package contractfinal

// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewKernelCheckCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewKernelCheckCommand(ctx)

	assert.Equal(t, "kernel-check", cmd.Use)
	assert.Equal(t, "Check Kernel Params", cmd.Short)
	assert.Equal(t, `Check Kernel Params, make sure satisfy the requirements`, cmd.Long)

	// Test command execution
	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)
}

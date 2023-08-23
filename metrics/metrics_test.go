// Copyright 2018 PingCAP, Inc.
//
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

package metrics

import (
	"testing"

	"github.com/pingcap/errors"
	"github.com/stretchr/testify/require"
	"github.com/wuhuizuo/tidb6/parser/terror"
)

func TestMetrics(_ *testing.T) {
	// Make sure it doesn't panic.
	PanicCounter.WithLabelValues(LabelDomain).Inc()
}

func TestRegisterMetrics(_ *testing.T) {
	// Make sure it doesn't panic.
	RegisterMetrics()
}

func TestRetLabel(t *testing.T) {
	require.Equal(t, opSucc, RetLabel(nil))
	require.Equal(t, opFailed, RetLabel(errors.New("test error")))
}

func TestExecuteErrorToLabel(t *testing.T) {
	require.Equal(t, `unknown`, ExecuteErrorToLabel(errors.New("test")))
	require.Equal(t, `global:2`, ExecuteErrorToLabel(terror.ErrResultUndetermined))
}

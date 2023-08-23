// Copyright 2019 PingCAP, Inc.
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

package aggfuncs_test

import (
	"testing"

	"github.com/wuhuizuo/tidb6/executor/aggfuncs"
	"github.com/wuhuizuo/tidb6/parser/ast"
	"github.com/wuhuizuo/tidb6/parser/mysql"
)

func TestMergePartialResult4BitFuncs(t *testing.T) {
	tests := []aggTest{
		buildAggTester(ast.AggFuncBitAnd, mysql.TypeLonglong, 5, 0, 0, 0),
		buildAggTester(ast.AggFuncBitOr, mysql.TypeLonglong, 5, 7, 7, 7),
		buildAggTester(ast.AggFuncBitXor, mysql.TypeLonglong, 5, 4, 5, 1),
	}
	for _, test := range tests {
		testMergePartialResult(t, test)
	}
}

func TestMemBitFunc(t *testing.T) {
	tests := []aggMemTest{
		buildAggMemTester(ast.AggFuncBitAnd, mysql.TypeLonglong, 5,
			aggfuncs.DefPartialResult4BitFuncSize, defaultUpdateMemDeltaGens, false),
		buildAggMemTester(ast.AggFuncBitOr, mysql.TypeLonglong, 5,
			aggfuncs.DefPartialResult4BitFuncSize, defaultUpdateMemDeltaGens, false),
		buildAggMemTester(ast.AggFuncBitXor, mysql.TypeLonglong, 5,
			aggfuncs.DefPartialResult4BitFuncSize, defaultUpdateMemDeltaGens, false),
	}
	for _, test := range tests {
		testAggMemFunc(t, test)
	}
}

// Copyright 2022 PingCAP, Inc.
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

package executor_test

import (
	"strings"
	"testing"

	"github.com/pingcap/failpoint"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/tipb/go-tipb"
	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/tikvrpc"
	"github.com/wuhuizuo/tidb6/config"
	"github.com/wuhuizuo/tidb6/parser"
	plannercore "github.com/wuhuizuo/tidb6/planner/core"
	"github.com/wuhuizuo/tidb6/store/mockstore/unistore"
	"github.com/wuhuizuo/tidb6/tablecodec"
	"github.com/wuhuizuo/tidb6/testkit"
	"github.com/wuhuizuo/tidb6/testkit/external"
	topsqlstate "github.com/wuhuizuo/tidb6/util/topsql/state"
)

func TestResourceGroupTag(t *testing.T) {
	store := testkit.CreateMockStore(t)

	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t;")
	tk.MustExec("create table t(a int, b int, unique index idx(a));")

	tbInfo := external.GetTableByName(t, tk, "test", "t")

	// Enable Top SQL
	topsqlstate.EnableTopSQL()
	config.UpdateGlobal(func(conf *config.Config) {
		conf.TopSQL.ReceiverAddress = "mock-agent"
	})

	require.NoError(t, failpoint.Enable("github.com/wuhuizuo/tidb6/store/mockstore/unistore/unistoreRPCClientSendHook", `return(true)`))
	defer func() {
		require.NoError(t, failpoint.Disable("github.com/wuhuizuo/tidb6/store/mockstore/unistore/unistoreRPCClientSendHook"))
	}()

	var sqlDigest, planDigest *parser.Digest
	var tagLabel tipb.ResourceGroupTagLabel
	checkFn := func() {}
	unistore.UnistoreRPCClientSendHook = func(req *tikvrpc.Request) {
		var startKey []byte
		var ctx *kvrpcpb.Context
		switch req.Type {
		case tikvrpc.CmdGet:
			request := req.Get()
			startKey = request.Key
			ctx = request.Context
		case tikvrpc.CmdBatchGet:
			request := req.BatchGet()
			startKey = request.Keys[0]
			ctx = request.Context
		case tikvrpc.CmdPrewrite:
			request := req.Prewrite()
			startKey = request.Mutations[0].Key
			ctx = request.Context
		case tikvrpc.CmdCommit:
			request := req.Commit()
			startKey = request.Keys[0]
			ctx = request.Context
		case tikvrpc.CmdCop:
			request := req.Cop()
			startKey = request.Ranges[0].Start
			ctx = request.Context
		case tikvrpc.CmdPessimisticLock:
			request := req.PessimisticLock()
			startKey = request.PrimaryLock
			ctx = request.Context
		}
		tid := tablecodec.DecodeTableID(startKey)
		if tid != tbInfo.Meta().ID {
			return
		}
		if ctx == nil {
			return
		}
		tag := &tipb.ResourceGroupTag{}
		err := tag.Unmarshal(ctx.ResourceGroupTag)
		require.NoError(t, err)
		sqlDigest = parser.NewDigest(tag.SqlDigest)
		planDigest = parser.NewDigest(tag.PlanDigest)
		tagLabel = *tag.Label
		checkFn()
	}

	resetVars := func() {
		sqlDigest = parser.NewDigest(nil)
		planDigest = parser.NewDigest(nil)
	}

	cases := []struct {
		sql       string
		tagLabels map[tipb.ResourceGroupTagLabel]struct{}
		ignore    bool
	}{
		{
			sql: "insert into t values(1,1),(2,2),(3,3)",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql: "select * from t use index (idx) where a=1",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelRow:   {},
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql: "select * from t use index (idx) where a in (1,2,3)",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelRow:   {},
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql: "select * from t use index (idx) where a>1",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelRow:   {},
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql: "select * from t where b>1",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelRow: {},
			},
		},
		{
			sql: "select a from t use index (idx) where a>1",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql:    "begin pessimistic",
			ignore: true,
		},
		{
			sql: "insert into t values(4,4)",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelRow:   {},
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql:    "commit",
			ignore: true,
		},
		{
			sql: "update t set a=5,b=5 where a=5",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
		{
			sql: "replace into t values(6,6)",
			tagLabels: map[tipb.ResourceGroupTagLabel]struct{}{
				tipb.ResourceGroupTagLabel_ResourceGroupTagLabelIndex: {},
			},
		},
	}
	for _, ca := range cases {
		resetVars()

		_, expectSQLDigest := parser.NormalizeDigest(ca.sql)
		var expectPlanDigest *parser.Digest
		checkCnt := 0
		checkFn = func() {
			if ca.ignore {
				return
			}
			if expectPlanDigest == nil {
				info := tk.Session().ShowProcess()
				require.NotNil(t, info)
				p, ok := info.Plan.(plannercore.Plan)
				require.True(t, ok)
				_, expectPlanDigest = plannercore.NormalizePlan(p)

				flat := plannercore.FlattenPhysicalPlan(p, false)
				_, newPlanDigest := plannercore.NormalizeFlatPlan(flat)
				require.Equal(t, expectPlanDigest, newPlanDigest)
			}
			require.Equal(t, sqlDigest.String(), expectSQLDigest.String(), "%v", ca.sql)
			require.Equal(t, planDigest.String(), expectPlanDigest.String())
			_, ok := ca.tagLabels[tagLabel]
			require.True(t, ok)
			checkCnt++
		}

		if strings.HasPrefix(ca.sql, "select") {
			tk.MustQuery(ca.sql)
		} else {
			tk.MustExec(ca.sql)
		}
		if ca.ignore {
			continue
		}
		require.Greater(t, checkCnt, 0, "%v", ca.sql)
	}
}

// Copyright 2021 PingCAP, Inc.
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

package expression

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/tikv"
	"github.com/wuhuizuo/tidb6/config"
	"github.com/wuhuizuo/tidb6/testkit/testdata"
	"github.com/wuhuizuo/tidb6/testkit/testmain"
	"github.com/wuhuizuo/tidb6/testkit/testsetup"
	"github.com/wuhuizuo/tidb6/util/mock"
	"github.com/wuhuizuo/tidb6/util/timeutil"
	"go.uber.org/goleak"
)

var testDataMap = make(testdata.BookKeeper)

func TestMain(m *testing.M) {
	testsetup.SetupForCommonTest()
	testmain.ShortCircuitForBench(m)

	config.UpdateGlobal(func(conf *config.Config) {
		conf.TiKVClient.AsyncCommit.SafeWindow = 0
		conf.TiKVClient.AsyncCommit.AllowedClockDrift = 0
		conf.Experimental.AllowsExpressionIndex = true
	})
	tikv.EnableFailpoints()

	// Some test depends on the values of timeutil.SystemLocation()
	// If we don't SetSystemTZ() here, the value would change unpredictable.
	// Affected by the order whether a testsuite runs before or after integration test.
	// Note, SetSystemTZ() is a sync.Once operation.
	timeutil.SetSystemTZ("system")

	testDataMap.LoadTestSuiteData("testdata", "flag_simplify")
	testDataMap.LoadTestSuiteData("testdata", "expression_suite")

	opts := []goleak.Option{
		goleak.IgnoreTopFunction("github.com/golang/glog.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("github.com/lestrrat-go/httprc.runFetchWorker"),
		goleak.IgnoreTopFunction("go.etcd.io/etcd/client/pkg/v3/logutil.(*MergeLogger).outputLoop"),
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
	}

	callback := func(i int) int {
		testDataMap.GenerateOutputIfNeeded()
		return i
	}
	goleak.VerifyTestMain(testmain.WrapTestingM(m, callback), opts...)
}

func createContext(t *testing.T) *mock.Context {
	ctx := mock.NewContext()
	ctx.GetSessionVars().StmtCtx.TimeZone = time.Local
	sc := ctx.GetSessionVars().StmtCtx
	sc.TruncateAsWarning = true
	require.NoError(t, ctx.GetSessionVars().SetSystemVar("max_allowed_packet", "67108864"))
	ctx.GetSessionVars().PlanColumnID = 0
	return ctx
}

func GetFlagSimplifyData() testdata.TestData {
	return testDataMap["flag_simplify"]
}

func GetExpressionSuiteData() testdata.TestData {
	return testDataMap["expression_suite"]
}
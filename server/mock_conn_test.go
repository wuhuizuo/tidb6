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

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wuhuizuo/tidb6/parser/mysql"
	"github.com/wuhuizuo/tidb6/testkit"
)

func TestMockConn(t *testing.T) {
	store := testkit.CreateMockStore(t)
	server := CreateMockServer(t, store)
	defer server.Close()
	conn := CreateMockConn(t, server)
	defer conn.Close()

	require.NoError(t, conn.HandleQuery(context.Background(), "select 1"))
	require.Equal(t, "select 1", conn.Context().GetSessionVars().StmtCtx.OriginalSQL)

	require.Error(t, conn.HandleQuery(context.Background(), "select"))

	inBytes := append([]byte{mysql.ComQuery}, []byte("select 1")...)
	require.NoError(t, conn.Dispatch(context.Background(), inBytes))
	require.Equal(t, "select 1", conn.Context().GetSessionVars().StmtCtx.OriginalSQL)

	inBytes = append([]byte{mysql.ComStmtPrepare}, []byte("select 1")...)
	require.NoError(t, conn.Dispatch(context.Background(), inBytes))
	require.Equal(t, "select 1", conn.Context().GetSessionVars().StmtCtx.OriginalSQL)
}

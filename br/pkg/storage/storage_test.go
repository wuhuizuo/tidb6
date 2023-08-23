// Copyright 2022 PingCAP, Inc. Licensed under Apache-2.0.

package storage_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wuhuizuo/tidb6/br/pkg/storage"
)

func TestDefaultHttpTransport(t *testing.T) {
	transport, ok := storage.CloneDefaultHttpTransport()
	require.True(t, ok)
	require.True(t, transport.MaxConnsPerHost == 0)
	require.True(t, transport.MaxIdleConns > 0)
}

func TestDefaultHttpClient(t *testing.T) {
	var concurrency uint = 128
	transport, ok := storage.GetDefaultHttpClient(concurrency).Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t, int(concurrency), transport.MaxIdleConnsPerHost)
	require.Equal(t, int(concurrency), transport.MaxIdleConns)
}

// Copyright 2021 PingCAP, Inc. Licensed under Apache-2.0.

package redact_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wuhuizuo/tidb6/br/pkg/redact"
	"go.uber.org/goleak"
)

func TestRedact(t *testing.T) {
	defer goleak.VerifyNone(t)

	redacted, secret := "?", "secret"

	redact.InitRedact(false)
	require.Equal(t, redact.String(secret), secret)
	require.Equal(t, redact.Key([]byte(secret)), hex.EncodeToString([]byte(secret)))

	redact.InitRedact(true)
	require.Equal(t, redact.String(secret), redacted)
	require.Equal(t, redact.Key([]byte(secret)), redacted)
}

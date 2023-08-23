// Copyright 2022 PingCAP, Inc. Licensed under Apache-2.0.
package split_test

import (
	"context"
	"testing"

	"github.com/pingcap/failpoint"
	"github.com/stretchr/testify/require"
	berrors "github.com/wuhuizuo/tidb6/br/pkg/errors"
	"github.com/wuhuizuo/tidb6/br/pkg/restore/split"
	"github.com/wuhuizuo/tidb6/br/pkg/utils"
)

func TestScanRegionBackOfferWithSuccess(t *testing.T) {
	var counter int
	bo := split.NewScanRegionBackoffer()

	err := utils.WithRetry(context.Background(), func() error {
		defer func() {
			counter++
		}()

		if counter == 3 {
			return nil
		}
		return berrors.ErrPDBatchScanRegion
	}, bo)
	require.NoError(t, err)
	require.Equal(t, counter, 4)
}

func TestScanRegionBackOfferWithFail(t *testing.T) {
	_ = failpoint.Enable("github.com/wuhuizuo/tidb6/br/pkg/restore/split/hint-scan-region-backoff", "return(true)")
	defer func() {
		_ = failpoint.Disable("github.com/wuhuizuo/tidb6/br/pkg/restore/split/hint-scan-region-backoff")
	}()

	var counter int
	bo := split.NewScanRegionBackoffer()

	err := utils.WithRetry(context.Background(), func() error {
		defer func() {
			counter++
		}()
		return berrors.ErrPDBatchScanRegion
	}, bo)
	require.Error(t, err)
	require.Equal(t, counter, split.ScanRegionAttemptTimes)
}

func TestScanRegionBackOfferWithStopRetry(t *testing.T) {
	_ = failpoint.Enable("github.com/wuhuizuo/tidb6/br/pkg/restore/split/hint-scan-region-backoff", "return(true)")
	defer func() {
		_ = failpoint.Disable("github.com/wuhuizuo/tidb6/br/pkg/restore/split/hint-scan-region-backoff")
	}()

	var counter int
	bo := split.NewScanRegionBackoffer()

	err := utils.WithRetry(context.Background(), func() error {
		defer func() {
			counter++
		}()

		if counter < 5 {
			return berrors.ErrPDBatchScanRegion
		}
		return berrors.ErrKVUnknown
	}, bo)
	require.Error(t, err)
	require.Equal(t, counter, 6)
}

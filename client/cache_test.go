package client

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTipCapCash(t *testing.T) {
	var (
		ctx  = context.Background()
		c, _ = NewClient(ctx, TestEndpoint, nil, WithTimeout(10))
	)

	require.True(t, c.tipCash.isExpired())

	tip, err := c.tipCash.GasTipCap(ctx, &c)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(1000000000), tip)

	require.False(t, c.tipCash.isExpired())
}

func TestBaseFeeCash(t *testing.T) {
	var (
		ctx    = context.Background()
		c, _   = NewClient(ctx, TestEndpoint, nil, WithTimeout(10))
		tip, _ = c.tipCash.GasTipCap(ctx, &c)
	)

	require.True(t, c.baseFeeCash.isExpired())

	tip, err := c.baseFeeCash.GasFee(ctx, &c, tip)
	require.NoError(t, err)
	// require.Equal(t, big.NewInt(3000000000), tip)

	require.False(t, c.baseFeeCash.isExpired())
}

func TestNonceCash(t *testing.T) {
	var (
		ctx  = context.Background()
		c, _ = NewClient(ctx, TestEndpoint, nil, WithTimeout(10))
	)

	n, err := c.nonceCash.Nonce(ctx, TestPrivKey3, &c)
	require.NoError(t, err)
	require.Equal(t, uint64(0), n)

	require.True(t, c.nonceCash.nonces.Contains(TestPrivKey3))
}

package client

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tak1827/go-cache/lru"
)

func TestTipCapCash(t *testing.T) {
	var (
		ctx      = context.Background()
		c, _     = NewClient(ctx, TestEndpoint, WithTimeout(10))
		tipCache = TipCapCash{}
	)

	require.True(t, tipCache.isExpired())

	tip, err := tipCache.GasTipCap(ctx, &c)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(1000000000), tip)

	require.False(t, tipCache.isExpired())
}

func TestBaseFeeCash(t *testing.T) {
	var (
		ctx      = context.Background()
		c, _     = NewClient(ctx, TestEndpoint, WithTimeout(10))
		tipCache = TipCapCash{}
		tip, _   = tipCache.GasTipCap(ctx, &c)
		base     = BaseFeeCash{}
	)

	require.True(t, base.isExpired())

	tip, err := base.GasFee(ctx, &c, tip)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(3000000000), tip)

	require.False(t, base.isExpired())
}

func TestNonceCash(t *testing.T) {
	var (
		ctx       = context.Background()
		c, _      = NewClient(ctx, TestEndpoint, WithTimeout(10))
		nonceCash = NonceCash{nonces: lru.NewCache(1024)}
	)

	n, err := nonceCash.Nonce(ctx, TestPrivKey, &c)
	require.NoError(t, err)
	require.Equal(t, uint64(0), n)

	require.True(t, nonceCash.nonces.Contains(TestPrivKey))
}

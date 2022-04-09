package client

import (
	"context"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"github.com/tak1827/go-cache/lru"
	"github.com/tak1827/nonce-incrementor/nonce"
)

const (
	TipCapCashTTL  = 1024 * 15 // 1024 block * 15 sec
	BaseFeeCashTTL = 1024 * 15 // 1024 block * 15 sec
)

type TipCapCash struct {
	gas       *big.Int
	expiredAt int64
}

func (c TipCapCash) isExpired() bool {
	return c.expiredAt <= time.Now().Unix()
}

func (c *TipCapCash) GasTipCap(ctx context.Context, client *Client) (*big.Int, error) {
	if !c.isExpired() {
		return c.gas, nil
	}

	tip, err := client.ethclient.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get suggestion")
	}

	c.gas = tip
	c.expiredAt = time.Now().Unix() + TipCapCashTTL

	return tip, err
}

type BaseFeeCash struct {
	base      *big.Int
	expiredAt int64
}

func (c BaseFeeCash) isExpired() bool {
	return c.expiredAt <= time.Now().Unix()
}

func (c *BaseFeeCash) GasFee(ctx context.Context, client *Client, tip *big.Int) (*big.Int, error) {
	var baseFee *big.Int
	if !c.isExpired() {
		baseFee = c.base
	} else {
		head, err := client.ethclient.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get block header")
		}
		baseFee = head.BaseFee
		c.base = baseFee
		c.expiredAt = time.Now().Unix() + BaseFeeCashTTL
	}

	// ref: https://github.com/ethereum/go-ethereum/blob/v1.10.17/accounts/abi/bind/base.go#L252
	return new(big.Int).Add(tip, new(big.Int).Mul(baseFee, big.NewInt(2))), nil
}

type NonceCash struct {
	nonces lru.LRUCache
}

func (c *NonceCash) Nonce(ctx context.Context, priv string, client *Client) (uint64, error) {
	v, ok := c.nonces.Get(priv)
	if ok {
		return v.(*nonce.Nonce).Increment()
	}

	n, err := nonce.NewNonce(ctx, client, priv, true)
	if err != nil {
		return 0, errors.Wrap(err, "failed to new nonce")
	}

	c.nonces.Add(priv, &n)

	return n.Current()
}

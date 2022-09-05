package client

import (
	"context"
	"fmt"
	"math/big"
	"sync"
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
	sync.Mutex
	gas       *big.Int
	expiredAt int64
}

func (c *TipCapCash) isExpired() bool {
	return c.expiredAt <= time.Now().Unix()
}

func (c *TipCapCash) GasTipCap(ctx context.Context, client *Client) (*big.Int, error) {
	c.Lock()
	expired := c.isExpired()
	gas := c.gas
	c.Unlock()

	if !expired {
		return gas, nil
	}

	tip, err := client.ethclient.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get suggestion")
	}

	c.Lock()
	c.gas = tip
	c.expiredAt = time.Now().Unix() + TipCapCashTTL
	c.Unlock()

	return tip, err
}

type BaseFeeCash struct {
	sync.Mutex
	base      *big.Int
	expiredAt int64
}

func (c *BaseFeeCash) isExpired() bool {
	return c.expiredAt <= time.Now().Unix()
}

func (c *BaseFeeCash) GasFee(ctx context.Context, client *Client, tip *big.Int) (*big.Int, error) {
	c.Lock()
	expired := c.isExpired()
	baseFee := c.base
	c.Unlock()

	if expired {
		head, err := client.ethclient.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get block header")
		}
		baseFee = head.BaseFee

		c.Lock()
		c.base = baseFee
		c.expiredAt = time.Now().Unix() + BaseFeeCashTTL
		c.Unlock()
	}

	// ref: https://github.com/ethereum/go-ethereum/blob/v1.10.17/accounts/abi/bind/base.go#L252
	return new(big.Int).Add(tip, new(big.Int).Mul(baseFee, big.NewInt(2))), nil
}

type NonceCash struct {
	sync.Mutex
	nonces lru.LRUCache
}

func (c *NonceCash) Nonce(ctx context.Context, priv string, client *Client) (uint64, error) {
	c.Lock()
	defer c.Unlock()

	v, ok := c.nonces.Get(priv)
	if ok {
		return v.(*nonce.Nonce).Increment()
	}

	ensure := true
	n, err := nonce.NewNonce(ctx, client, priv, ensure)
	if err != nil {
		return 0, errors.Wrap(err, "failed to new nonce")
	}

	c.nonces.Add(priv, n)

	return n.Increment()
}

func (c *NonceCash) Decrement(ctx context.Context, priv string, client *Client) error {
	c.Lock()
	defer c.Unlock()

	v, ok := c.nonces.Get(priv)
	if !ok {
		return fmt.Errorf("no nonce for %s", priv)
	}

	_, err := v.(*nonce.Nonce).Decrement()
	return err
}

func (c *NonceCash) Reset(priv string, n uint64) error {
	c.Lock()
	defer c.Unlock()

	v, ok := c.nonces.Get(priv)
	if !ok {
		return fmt.Errorf("no nonce for %s", priv)
	}

	v.(*nonce.Nonce).Reset(n)
	return nil
}

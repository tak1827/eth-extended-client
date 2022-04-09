package client

import (
	"math/big"
)

const (
	DefaultGasPrice = int64(0)  // 1 gwai
	DefaultTimeout  = int64(60) // 60 sec
)

type Option interface {
	Apply(*Client)
}

type GasPriceOpt int64

func (o GasPriceOpt) Apply(c *Client) {
	c.GasPrice = big.NewInt(int64(o))
}
func WithGasPrice(gasPrice int64) GasPriceOpt {
	return GasPriceOpt(gasPrice)
}

type TimeoutOpt int64

func (t TimeoutOpt) Apply(c *Client) {
	c.timeout = int64(t)
}
func WithTimeout(t int64) TimeoutOpt {
	if t <= 0 {
		panic("Timeout should be positive")
	}
	return TimeoutOpt(t)
}

package client

import (
	"math/big"
	"os"

	"github.com/rs/zerolog"
)

const (
	DefaultGasPrice                = int64(0)      // 1 gwai
	DefaultTimeout                 = int64(60)     // 60 sec
	DefaultSyncSendTimeout         = int64(60 * 3) // 180 sec
	DefaultSyncSendConfirmInterval = int64(1000)   // 1s
	DefaultConfirmerQueueSize      = 4096
)

var DefaultLogger = zerolog.New(os.Stderr).Level(zerolog.InfoLevel).With().Timestamp().Logger()

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

type SyncSendTimeoutOpt int64

func (t SyncSendTimeoutOpt) Apply(c *Client) {
	c.syncSendTimeout = int64(t)
}
func WithSyncSendTimeout(t int64) SyncSendTimeoutOpt {
	if t <= 0 {
		panic("SyncSendTimeout should be positive")
	}
	return SyncSendTimeoutOpt(t)
}

type ConfirmerQueueSizeOpt int64

func (o ConfirmerQueueSizeOpt) Apply(c *Client) {
	c.queueSize = int(o)
}
func WithConfirmerQueueSize(size int) ConfirmerQueueSizeOpt {
	return ConfirmerQueueSizeOpt(size)
}

type SyncSendConfirmIntervalOpt int64

func (o SyncSendConfirmIntervalOpt) Apply(c *Client) {
	c.syncSendConfirmInterval = int64(o)
}
func WithSyncSendConfirmInterval(size int) SyncSendConfirmIntervalOpt {
	return SyncSendConfirmIntervalOpt(size)
}

type LoggerOpt zerolog.Logger

func (o LoggerOpt) Apply(c *Client) {
	c.logger = zerolog.Logger(o)
}
func WithLoggerOpt(logger zerolog.Logger) LoggerOpt {
	return LoggerOpt(logger)
}

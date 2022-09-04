package client

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/tak1827/go-cache/lru"
	"github.com/tak1827/transaction-confirmer/confirm"
)

var (
	ErrSyncSendTimeout = errors.New("sync send timeout")

	timeoutDuration                 time.Duration
	syncSendTimeoutDuration         time.Duration
	syncSendConfirmIntervalDuration time.Duration
)

type Client struct {
	ethclient *ethclient.Client

	GasPrice *big.Int
	chainID  *big.Int
	timeout  int64

	tipCash     *TipCapCash
	baseFeeCash *BaseFeeCash
	nonceCash   *NonceCash

	logger zerolog.Logger

	confirmer               *confirm.Confirmer
	queueSize               int
	unconfirmedTx           *safeMap
	sentTx                  *safeMap
	syncSendTimeout         int64
	syncSendConfirmInterval int64
	cancel                  context.CancelFunc
}

func NewClient(ctx context.Context, endpoint string, cfmOpts []confirm.Opt, opts ...Option) (c Client, err error) {
	rpcclient, err := rpc.DialContext(ctx, endpoint)
	if err != nil {
		err = errors.Wrapf(err, "failed to conecting endpoint(%s)", endpoint)
		return
	}

	c.ethclient = ethclient.NewClient(rpcclient)
	c.GasPrice = big.NewInt(int64(DefaultGasPrice))
	c.timeout = DefaultTimeout
	c.tipCash = &TipCapCash{}
	c.baseFeeCash = &BaseFeeCash{}
	c.nonceCash = &NonceCash{nonces: lru.NewCache(1024, 0)}
	c.queueSize = DefaultConfirmerQueueSize
	c.logger = DefaultLogger
	c.syncSendTimeout = DefaultSyncSendTimeout
	c.syncSendConfirmInterval = DefaultSyncSendConfirmInterval

	if c.chainID, err = c.ethclient.ChainID(ctx); err != nil {
		err = errors.Wrap(err, "failed to get chain id")
		return
	}

	for i := range opts {
		opts[i].Apply(&c)
	}

	confirmer := confirm.NewConfirmer(&c, c.queueSize, append([]confirm.Opt{
		confirm.WithWorkers(1),
		confirm.WithTimeout(c.timeout),
		confirm.WithAfterTxSent(c.afterTxSent),
		confirm.WithAfterTxConfirmed(c.afterTxConfirmed),
	}, cfmOpts...)...)

	c.confirmer = &confirmer
	c.unconfirmedTx = &safeMap{item: make(map[string]struct{})}
	c.sentTx = &safeMap{item: make(map[string]struct{})}

	timeoutDuration = time.Duration(time.Duration(c.timeout) * time.Second)
	syncSendTimeoutDuration = time.Duration(time.Duration(c.syncSendTimeout) * time.Second)
	syncSendConfirmIntervalDuration = time.Duration(time.Duration(c.syncSendConfirmInterval) * time.Millisecond)

	return
}

func (c *Client) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	c.confirmer.Start(ctx)
}

func (c *Client) Stop() {
	c.confirmer.Close(c.cancel)
	c.ethclient.Close()
}

func (c *Client) Nonce(ctx context.Context, priv string) (nonce uint64, err error) {
	privKey, err := crypto.HexToECDSA(priv)
	if err != nil {
		err = errors.Wrap(err, "failed to get nonce")
		return
	}

	account := crypto.PubkeyToAddress(privKey.PublicKey)
	nonce, err = c.ethclient.NonceAt(ctx, account, nil)
	return
}

func (c *Client) SendTx(ctx context.Context, tx interface{}) (string, error) {
	signedTx := tx.(*types.Transaction)

	if err := c.ethclient.SendTransaction(ctx, signedTx); err != nil {
		return "", errors.Wrap(err, "err SendTransaction")
	}

	return signedTx.Hash().Hex(), nil
}

func (c *Client) AsyncSend(ctx context.Context, priv string, to *common.Address, amount *big.Int, input []byte, gasLimit uint64) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	tx, err := c.sinedTx(timeoutCtx, priv, to, amount, input, gasLimit)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign tx")
	}

	return c.SendTx(timeoutCtx, tx)
}

func (c *Client) SyncSend(ctx context.Context, priv string, to *common.Address, amount *big.Int, input []byte, gasLimit uint64) (hash string, err error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	tx, err := c.sinedTx(timeoutCtx, priv, to, amount, input, gasLimit)
	if err != nil {
		err = errors.Wrap(err, "failed to sign tx")
		return
	}

	hash = tx.Hash().Hex()

	if err = c.confirmer.EnqueueTx(timeoutCtx, tx); err != nil {
		err = errors.Wrapf(err, "failed to enqueue tx(%v)", tx)
		return
	}

	timeoutCtx, cancel = context.WithTimeout(ctx, syncSendTimeoutDuration)
	defer cancel()

	timer := time.NewTicker(syncSendConfirmIntervalDuration)
	defer timer.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			err = ErrSyncSendTimeout
			return
		case <-timer.C:
			if !c.sentTx.has(hash) {
				continue
			}
			if !c.unconfirmedTx.has(hash) {
				c.sentTx.delete(hash)
				return
			}
		}
	}
}

func (c *Client) ConfirmTx(ctx context.Context, hash string, confirmationBlocks uint64) error {
	recept, err := c.Receipt(ctx, hash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return confirm.ErrTxNotFound
		}
		return errors.Wrap(err, "err TransactionReceipt")
	}

	if recept.Status != 1 {
		return confirm.ErrTxFailed
	}

	block, err := c.LatestBlockNumber(ctx)
	if err != nil {
		return errors.Wrap(err, "err LatestBlockNumber")
	}

	if recept.BlockNumber.Uint64()+confirmationBlocks > block {
		return confirm.ErrTxConfirmPending
	}

	return nil
}

func (c *Client) Receipt(ctx context.Context, hash string) (*types.Receipt, error) {
	return c.ethclient.TransactionReceipt(ctx, common.HexToHash(hash))
}

func (c *Client) BalanceOf(ctx context.Context, account common.Address) (*big.Int, error) {
	return c.ethclient.BalanceAt(ctx, account, nil)
}

func (c *Client) LatestBlockNumber(ctx context.Context) (uint64, error) {
	header, err := c.ethclient.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Uint64(), nil
}

func (c *Client) QueryContract(ctx context.Context, to common.Address, input []byte) (output []byte, err error) {
	var (
		msg  = ethereum.CallMsg{To: &to, Data: input}
		code []byte
	)
	if output, err = c.ethclient.CallContract(ctx, msg, nil); err != nil {
		err = errors.Wrapf(err, "failed to call contract(=%s)", to.String())
		return
	}

	if len(output) == 0 {
		// Make sure we have a contract to operate on, and bail out otherwise.
		if code, err = c.ethclient.CodeAt(ctx, to, nil); err != nil {
			err = errors.Wrap(err, "at ethclient.CodeAt")
			return
		} else if len(code) == 0 {
			err = errors.Wrap(bind.ErrNoCode, "at ethclient.CodeAt")
			return
		}
	}

	return
}

func (c *Client) estimateGasLimit(ctx context.Context, from common.Address, to *common.Address, value *big.Int, input []byte, tip, gasFee *big.Int) (uint64, error) {
	msg := ethereum.CallMsg{
		From:      from,
		To:        to,
		GasPrice:  c.GasPrice,
		GasTipCap: tip,
		GasFeeCap: gasFee,
		Value:     value,
		Data:      input,
	}
	return c.ethclient.EstimateGas(ctx, msg)
}

func (c *Client) sinedTx(ctx context.Context, priv string, to *common.Address, amount *big.Int, input []byte, gasLimit uint64) (*types.Transaction, error) {
	privKey, err := crypto.HexToECDSA(priv)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nonce")
	}

	tip, err := c.tipCash.GasTipCap(ctx, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GasTipCap")
	}

	gasFee, err := c.baseFeeCash.GasFee(ctx, c, tip)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get FeeCap")
	}

	if gasLimit == 0 {
		auth := bind.NewKeyedTransactor(privKey)
		if gasLimit, err = c.estimateGasLimit(ctx, auth.From, to, amount, input, tip, gasFee); err != nil {
			return nil, errors.Wrap(err, "failed to estimate gas")
		}
	}

	n, err := c.nonceCash.Nonce(ctx, priv, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nonce")
	}

	txdata := &types.DynamicFeeTx{
		ChainID:    c.chainID,
		Nonce:      n,
		GasTipCap:  tip,
		GasFeeCap:  gasFee,
		Gas:        gasLimit,
		To:         to,
		Value:      amount,
		Data:       input,
		AccessList: nil,
	}

	signer := types.NewLondonSigner(c.chainID)

	return types.SignNewTx(privKey, signer, txdata)
}

func (c *Client) afterTxSent(hash string) error {
	c.logger.Info().Msgf("tx sent, hash: %s", hash)
	c.unconfirmedTx.add(hash)
	c.sentTx.add(hash)
	return nil
}

func (c *Client) afterTxConfirmed(hash string) error {
	c.logger.Info().Msgf("tx confirmed, tx: %v", hash)
	if c.unconfirmedTx.has(hash) {
		c.unconfirmedTx.delete(hash)
	}
	return nil
}

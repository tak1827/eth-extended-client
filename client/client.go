package client

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/tak1827/go-cache/lru"
)

type Client struct {
	ethclient *ethclient.Client

	GasPrice *big.Int
	chainID  *big.Int
	timeout  int64

	tipCash     TipCapCash
	baseFeeCash BaseFeeCash
	nonceCash   NonceCash
}

func NewClient(ctx context.Context, endpoint string, opts ...Option) (c Client, err error) {
	rpcclient, err := rpc.DialContext(ctx, endpoint)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("failed to conecting endpoint(%s)", endpoint))
		return
	}

	c.ethclient = ethclient.NewClient(rpcclient)
	c.GasPrice = big.NewInt(int64(DefaultGasPrice))
	c.timeout = DefaultTimeout

	if c.chainID, err = c.ethclient.ChainID(ctx); err != nil {
		err = errors.Wrap(err, "failed to get chain id")
		return
	}

	c.tipCash = TipCapCash{}
	c.baseFeeCash = BaseFeeCash{}
	c.nonceCash = NonceCash{nonces: lru.NewCache(1024)}

	for i := range opts {
		opts[i].Apply(&c)
	}

	return
}

func (c *Client) Nonce(ctx context.Context, privKey string) (nonce uint64, err error) {
	priv, err := crypto.HexToECDSA(privKey)
	if err != nil {
		err = errors.Wrap(err, "failed to get nonce")
		return
	}

	account := crypto.PubkeyToAddress(priv.PublicKey)
	nonce, err = c.ethclient.NonceAt(ctx, account, nil)
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

func (c *Client) buildSinedTx(ctx context.Context, priv string, to *common.Address, amount *big.Int, input []byte) (*types.Transaction, error) {
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

	auth := bind.NewKeyedTransactor(privKey)
	gasLimit, err := c.estimateGasLimit(ctx, auth.From, to, amount, input, tip, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to estimate gas")
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

// func (c *Client) SyncSend(ctx context.Context, from, to *common.Address, input []byte, value *big.Int) {

// }

// func (c *Client) AsyncSend()

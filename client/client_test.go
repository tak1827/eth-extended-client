package client

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tak1827/eth-extended-client/contract"
	"github.com/tak1827/transaction-confirmer/confirm"
)

const (
	TestEndpoint = "http://localhost:8545"
	TestPrivKey  = "d1c71e71b06e248c8dbe94d49ef6d6b0d64f5d71b1e33a0f39e14dadb070304a"
	// TestAccount  = "0xE3b0DE0E4CA5D3CB29A9341534226C4D31C9838f"
	TestAccount = "0x26fa9f1a6568b42e29b1787c403B3628dFC0C6FE"
)

// func TestAsyncSend(t *testing.T) {
// 	var (
// 		ctx    = context.Background()
// 		c, _   = NewClient(ctx, TestEndpoint, nil, WithTimeout(10))
// 		to, _  = GenerateAddr()
// 		amount = ToWei(1.0, 9) // 1gwai
// 	)

// 	// send eth
// 	_, err := c.AsyncSend(ctx, TestPrivKey, &to, amount, nil)
// 	require.NoError(t, err)

// 	// deploy contract
// 	var (
// 		parsed, _ = abi.JSON(strings.NewReader(contract.ERC20ABI))
// 		input, _  = parsed.Pack("", []interface{}{"name", "symbol"}...)
// 		bytecode  = common.FromHex(contract.ERC20Bin)
// 	)
// 	_, err = c.AsyncSend(ctx, TestPrivKey, nil, nil, append(bytecode, input...))
// 	require.NoError(t, err)
// }

func TestSyncSend(t *testing.T) {
	var (
		ctx     = context.Background()
		cfmOpts = []confirm.Opt{
			confirm.WithWorkers(1),
			confirm.WithWorkerInterval(64),
			confirm.WithConfirmationBlock(0),
		}
		c, _   = NewClient(ctx, TestEndpoint, cfmOpts, WithTimeout(10), WithSyncSendConfirmInterval(64))
		to, _  = GenerateAddr()
		amount = ToWei(1.0, 9) // 1gwai
	)

	c.Start()
	defer c.Stop()

	// send eth
	_, err := c.SyncSend(ctx, TestPrivKey, &to, amount, nil, 0)
	require.NoError(t, err)

	// deploy contract
	var (
		parsed, _ = abi.JSON(strings.NewReader(contract.ERC20ABI))
		input, _  = parsed.Pack("", []interface{}{"name", "symbol"}...)
		bytecode  = common.FromHex(contract.ERC20Bin)
	)
	hash, err := c.SyncSend(ctx, TestPrivKey, nil, nil, append(bytecode, input...), 0)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// mint token
	var (
		receipt, _  = c.Receipt(ctx, hash)
		account     = common.HexToAddress(TestAccount)
		issInput, _ = parsed.Pack("mint", []interface{}{account, amount}...)
	)

	_, err = c.SyncSend(ctx, TestPrivKey, &receipt.ContractAddress, nil, issInput, 0)
	require.NoError(t, err)

	var (
		balInput, _ = parsed.Pack("balanceOf", []interface{}{account}...)
	)
	output, err := c.QueryContract(ctx, receipt.ContractAddress, balInput)
	require.NoError(t, err)

	var (
		results, _ = parsed.Unpack("balanceOf", output)
		balance    = *abi.ConvertType(results[0], new(*big.Int)).(**big.Int)
	)
	require.Equal(t, amount.String(), balance.String())
}

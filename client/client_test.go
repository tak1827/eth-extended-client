package client

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	// "github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tak1827/eth-extended-client/contract"
	"github.com/tak1827/transaction-confirmer/confirm"
)

const (
	TestEndpoint = "http://localhost:8545"
	TestPrivKey  = "d1c71e71b06e248c8dbe94d49ef6d6b0d64f5d71b1e33a0f39e14dadb070304a"
	TestAccount  = "0xE3b0DE0E4CA5D3CB29A9341534226C4D31C9838f"
	TestPrivKey2 = "8179ce3d00ac1d1d1d38e4f038de00ccd0e0375517164ac5448e3acc847acb34"
	TestAccount2 = "0x26fa9f1a6568b42e29b1787c403B3628dFC0C6FE"
	TestPrivKey3 = "df38daebd09f56398cc8fd699b72f5ea6e416878312e1692476950f427928e7d"
	TestAccount3 = "0x31a6EE302c1E7602685c86EF7a3069210Bc26670"
	// testnet
	TestNetEndpoint = "https://rinkeby.infura.io/v3/b3dd59dcade64d8d9d7b5dbfe403c152"
	TestNetPrivKey  = "d9a595ce8dbd72830662a7cc4cc85931e8c5311dd19c54f9c36e8080db12702f"
	TestNetAccount  = "0xC9911Ccf8FacBA9D7D8f1C59FE477233b6Bb9fE4"
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
		account     = common.HexToAddress(TestAccount2)
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

func TestConcurrent(t *testing.T) {
	var (
		ctx     = context.Background()
		cfmOpts = []confirm.Opt{
			confirm.WithWorkers(2),
			confirm.WithWorkerInterval(64),
			confirm.WithConfirmationBlock(0),
		}
		c, _  = NewClient(ctx, TestEndpoint, cfmOpts, WithTimeout(10), WithSyncSendConfirmInterval(128))
		to, _ = GenerateAddr()
		// dummy, _ = GenerateAddr()
		amount   = ToWei(1.0, 9) // 1gwai
		size     = 6
		gasLimit = uint64(0)
		wg       sync.WaitGroup
	)

	c.Start()
	defer c.Stop()

	for i := 0; i < size; i++ {
		wg.Add(1)

		if i%2 == 0 {
			gasLimit = 10000
		} else {
			gasLimit = 0
		}
		go func(a *big.Int, l uint64) {
			defer wg.Done()

			_, err := c.SyncSend(context.Background(), TestPrivKey2, &to, a, nil, l)
			if l == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		}(amount, gasLimit)
	}

	for i := 0; i < size/2; i++ {
		_, _ = c.AsyncSend(context.Background(), TestPrivKey2, &to, amount, nil, 0)
	}

	wg.Wait()

	time.Sleep(2 * time.Second)

	balance, err := c.BalanceOf(ctx, to)
	require.NoError(t, err)

	expected := big.NewInt(int64(size))
	expected.Mul(expected, amount)
	require.Equal(t, expected.String(), balance.String())
}

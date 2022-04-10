package client

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	// "github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tak1827/eth-extended-client/contract"
)

const (
	TestEndpoint = "http://localhost:8545"
	TestPrivKey  = "d1c71e71b06e248c8dbe94d49ef6d6b0d64f5d71b1e33a0f39e14dadb070304a"
)

func TestAsyncSend(t *testing.T) {
	var (
		ctx    = context.Background()
		c, _   = NewClient(ctx, TestEndpoint, nil, WithTimeout(10))
		to, _  = GenerateAddr()
		amount = ToWei(1.0, 9) // 1gwai
	)

	// send eth
	_, err := c.AsyncSend(ctx, TestPrivKey, &to, amount, nil)
	require.NoError(t, err)

	// deploy contract
	var (
		parsed, _ = abi.JSON(strings.NewReader(contract.ERC20ABI))
		input, _  = parsed.Pack("", []interface{}{"name", "symbol"}...)
		bytecode  = common.FromHex(contract.ERC20Bin)
	)
	_, err = c.AsyncSend(ctx, TestPrivKey, nil, nil, append(bytecode, input...))
	require.NoError(t, err)
}

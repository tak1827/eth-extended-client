# eth-extended-client
The extended Ethereum client
- support sync send, confirming x blocks mined
- support async send

# Sample
```go
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

hash, err := c.SyncSend(ctx, TestPrivKey, &to, amount, nil)
if err != nil {
	panic(err)
}
fmt.Printf("tx: %s\n", hash)
```

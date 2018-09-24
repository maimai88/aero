package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"github.com/airbloc/aero/core"
	"github.com/airbloc/aero/operator"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethutils "github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/frostornge/ethornge/utils"
	"gopkg.in/urfave/cli.v1"
	"os"
	"runtime"
)

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = clientName
	app.Version = clientVersion
	app.Usage = "Aero sidechain network daemon"
	app.Author = ""
	app.Email = ""
	app.Action = startNode
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "verbosity",
			Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
			Value: 3,
		},
		cli.StringFlag{
			Name:  "genesis",
			Value: "",
			Usage: "Genesis block information generated from puppeth.",
		},
		cli.StringFlag{
			Name:  "parent",
			Value: "http://localhost:8545",
			Usage: "Accessible URI of the parent chain. Supports HTTP, WS, RPC.",
		},
		cli.StringFlag{
			Name:  "parent_bridge",
			Value: "0x00000000000000000000",
			Usage: "Bridge contract (ParentChain) address on the parent chain.",
		},
		cli.StringFlag{
			Name:  "host",
			Value: "localhost",
			Usage: "HTTP host for providing JSON RPC. Use 0.0.0.0 for public access.",
		},
		cli.IntFlag{
			Name:  "port",
			Value: 7600,
			Usage: "HTTP port for providing JSON RPC.",
		},
		cli.StringFlag{
			Name:  "wshost",
			Value: "localhost",
			Usage: "WebSocket host for providing real-time rpc.",
		},
		cli.IntFlag{
			Name:  "wsport",
			Value: 7601,
			Usage: "WebSocket port for providing real-time rpc.",
		},
		cli.IntFlag{
			Name:  "threads",
			Value: 0,
			Usage: "Number of threads that will execute transactions. (0=NumCPU)",
		},
	}
	return app
}

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func startNode(ctx *cli.Context) error {
	// setup logger
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(true)))
	glogger.Verbosity(log.Lvl(ctx.Int("verbosity")))
	log.Root().SetHandler(glogger)

	stack, err := setupNode(ctx)
	if err != nil {
		return err
	}

	ethutils.StartNode(stack)

	var ethereum *eth.Ethereum
	if err := stack.Service(&ethereum); err != nil {
		return fmt.Errorf("ethereum service not running: %v", err)
	}

	// Set the gas price to minimum for convenience.
	ethereum.TxPool().SetGasPrice(utils.Gwei(0))

	threads := ctx.Int("threads")
	if threads == 0 {
		threads = runtime.NumCPU()
	}
	if err := ethereum.StartMining(threads); err != nil {
		return fmt.Errorf("failed to start mining: %v", err)
	}

	privateKey, _ := ecdsa.GenerateKey(crypto.S256(), rand.Reader)

	// setup aero
	aero, err := core.NewAero(
		"ws://"+stack.WSEndpoint(),
		ctx.String("parent"),
		common.HexToAddress("0x00000000000000000000"),
		common.HexToAddress(ctx.String("parent_bridge")),
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Aero : %v", err)
	}

	// start operator
	op := operator.New(aero)
	go op.Start()

	// wait for termination :P
	stack.Wait()

	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	app := newApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

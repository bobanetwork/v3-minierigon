package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime/pprof"
	"strconv"

	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/consensus/ethash"
	"github.com/ledgerwatch/erigon/consensus/misc"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/oracle"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rlp"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if len(os.Args) > 2 {
		f, err := os.Create(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// non mips
	if len(os.Args) > 1 {
		newNodeUrl, setNewNodeUrl := os.LookupEnv("NODE")
		if setNewNodeUrl {
			fmt.Println("override node url", newNodeUrl)
			oracle.SetNodeUrl(newNodeUrl)
		}
		basedir := os.Getenv("BASEDIR")
		if len(basedir) == 0 {
			basedir = "/tmp/cannon"
		}

		// TODO
		// pkw := oracle.PreimageKeyValueWriter{}
		// pkwtrie := trie.NewStackTrie(pkw)

		blockNumber, _ := strconv.Atoi(os.Args[1])
		// TODO: get the chainid
		oracle.SetRoot(fmt.Sprintf("%s/0_%d", basedir, blockNumber))
		oracle.PrefetchBlock(big.NewInt(int64(blockNumber)), true)
		oracle.PrefetchBlock(big.NewInt(int64(blockNumber)+1), false)
		// hash, err := pkwtrie.Commit()
		// check(err)
		// fmt.Println("committed transactions", hash, err)
	}

	// init secp256k1BytePoints
	crypto.S256()

	// get inputs
	inputBytes := oracle.Preimage(oracle.InputHash())
	var inputs [6]common.Hash
	for i := 0; i < len(inputs); i++ {
		inputs[i] = common.BytesToHash(inputBytes[i*0x20 : i*0x20+0x20])
	}
	// read start block header
	var parent types.Header
	check(rlp.DecodeBytes(oracle.Preimage(inputs[0]), &parent))

	// read header
	var newheader types.Header
	// from parent
	newheader.ParentHash = parent.Hash()
	newheader.Number = big.NewInt(0).Add(parent.Number, big.NewInt(1))
	newheader.BaseFee = misc.CalcBaseFee(params.MainnetChainConfig, &parent)

	// from input oracle
	newheader.TxHash = inputs[1]
	newheader.Coinbase = common.BigToAddress(inputs[2].Big())
	newheader.UncleHash = inputs[3]
	newheader.GasLimit = inputs[4].Big().Uint64()
	newheader.Time = inputs[5].Big().Uint64()

	chainConfig := chain.Config{ChainID: big.NewInt(288)}
	newheader.Difficulty = ethash.CalcDifficulty(&chainConfig, newheader.Time, parent.Time, parent.Difficulty, parent.Number.Uint64(), parent.UncleHash)

	var txs []types.Transaction
	// TODO
	// find the transactions in the trie
	// triedb := oracle.NewDatabase(parent.Number, parent.Root)
	// tt := trie.New(newheader.TxHash)

	var uncles []*types.Header
	check(rlp.DecodeBytes(oracle.Preimage(newheader.UncleHash), &uncles))

	var receipts []*types.Receipt
	withdrawal := make([]*types.Withdrawal, 0, 0)
	block := types.NewBlock(&newheader, txs, uncles, receipts, withdrawal)
	fmt.Println("made block, parent:", newheader.ParentHash)

	if newheader.TxHash != block.Header().TxHash {
		panic("wrong transactions for block")
	}
	if newheader.UncleHash != block.Header().UncleHash {
		panic("wrong uncles for block " + newheader.UncleHash.String() + " " + block.Header().UncleHash.String())
	}

	// TODO
	// get transaction receipt and compare
}

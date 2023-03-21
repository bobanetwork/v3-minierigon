package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime/pprof"
	"strconv"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/oracle"
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

		// pkw := oracle.PreimageKeyValueWriter{}
		// pkwtrie := trie.NewStackTrie(pkw)

		blockNumber, _ := strconv.Atoi(os.Args[1])
		// TODO: get the chainid
		fmt.Println("blockNumber", blockNumber)
		oracle.SetRoot(fmt.Sprintf("%s/0_%d", basedir, blockNumber))
		oracle.PrefetchBlock(big.NewInt(int64(blockNumber)), true)
		oracle.PrefetchBlock(big.NewInt(int64(blockNumber)+1), false)
		// hash, err := pkwtrie.Commit()
		// check(err)
		// fmt.Println("committed transactions", hash, err)
	}

	// mips
	// init secp256k1BytePoints
	crypto.S256()

	// get inputs
	inputBytes := oracle.Preimage(oracle.InputHash())
	fmt.Println("inputBytes:", inputBytes)
	var inputs [6]common.Hash
	for i := 0; i < len(inputs); i++ {
		inputs[i] = common.BytesToHash(inputBytes[i*0x20 : i*0x20+0x20])
	}

	fmt.Println("Done!")
}

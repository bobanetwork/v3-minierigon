package oracle

import (
	"math/big"
	"sync"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
)

type Database struct {
	BlockNumber *big.Int
	Root        libcommon.Hash
	lock        sync.RWMutex
}

func NewDatabase(blockNumber *big.Int, root libcommon.Hash) Database {
	triedb := Database{BlockNumber: blockNumber, Root: root}
	//triedb.preimages = make(map[common.Hash][]byte)
	//fmt.Println("init database")
	PrefetchAccount(blockNumber, libcommon.Address{}, nil)

	//panic("preseed")
	return triedb
}

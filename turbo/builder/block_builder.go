package builder

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/log/v3"
	//"github.com/ledgerwatch/erigon/common/hexutil"
)

type BlockBuilderFunc func(param *core.BlockBuilderParameters, interrupt *int32) (*types.Block, error)

// BlockBuilder wraps a goroutine that builds Proof-of-Stake payloads (PoS "mining")
type BlockBuilder struct {
	interrupt int32
	syncCond  *sync.Cond
	block     *types.Block
	err       error
}

func NewBlockBuilder(build BlockBuilderFunc, param *core.BlockBuilderParameters) *BlockBuilder {
	b := new(BlockBuilder)
	b.syncCond = sync.NewCond(new(sync.Mutex))

	go func() {
		log.Info("Building block...")
		t := time.Now()
		block, err := build(param, &b.interrupt)
		if err != nil {
			log.Warn("Failed to build a block", "err", err)
		} else {
			log.Info("Built block", "hash", block.Hash(), "height", block.NumberU64(), "txs", len(block.Transactions()), "gas used %", 100*float64(block.GasUsed())/float64(block.GasLimit()), "time", time.Since(t))
		}

		b.syncCond.L.Lock()
		defer b.syncCond.L.Unlock()
		b.block = block
		b.err = err
		b.syncCond.Broadcast()
	}()

	return b
}

func NewBlockBuilderMM(
	build BlockBuilderFunc,
	param *core.BlockBuilderParameters,
	emptyHeader *types.Header,
	deposits [][]byte,
	noTxPool bool,
	mmChan chan int,
	) *BlockBuilder {

	b := new(BlockBuilder)
	b.emptyHeader = emptyHeader
	b.syncCond = sync.NewCond(new(sync.Mutex))
        
        param.Deposits = deposits
	param.NoTxPool = noTxPool
        log.Debug("MMDBG NewBlockBuilderMM", "deposits", deposits, "param", param)

	go func() {
		block, err := build(param, &b.interrupt)
		b.syncCond.L.Lock()
		defer b.syncCond.L.Unlock()
		b.block = block
		b.err = err
		log.Debug("MMDBG NewBlockBuilderMM notifying mmChan")
		mmChan <- 1
		log.Debug("MMDBG NewBlockBuilderMM broadcasting syncCond", "b", b)
		b.syncCond.Broadcast()
	}()
	log.Debug("MMDBG NewBlockBuilderMM returning", "b", b)
	return b
}

func (b *BlockBuilder) Stop() (*types.Block, error) {
	atomic.StoreInt32(&b.interrupt, 1)

	b.syncCond.L.Lock()
	defer b.syncCond.L.Unlock()
	for b.block == nil && b.err == nil {
		b.syncCond.Wait()
	}

	return b.block, b.err
}

func (b *BlockBuilder) Block() *types.Block {
	b.syncCond.L.Lock()
	defer b.syncCond.L.Unlock()

	return b.block
}

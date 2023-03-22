//go:build !mips
// +build !mips

package oracle

import (
	"math/big"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	types2 "github.com/ledgerwatch/erigon-lib/types"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/types"
)

// SendTxArgs represents the arguments to submit a transaction
// This struct is identical to ethapi.TransactionArgs, except for the usage of
// common.MixedcaseAddress in From and To
type SendTxArgs struct {
	From                 common.MixedcaseAddress  `json:"from"`
	To                   *common.MixedcaseAddress `json:"to"`
	Gas                  hexutil.Uint64           `json:"gas"`
	GasPrice             *hexutil.Big             `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big             `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big             `json:"maxPriorityFeePerGas"`
	Value                hexutil.Big              `json:"value"`
	Nonce                hexutil.Uint64           `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input,omitempty"`

	// For non-legacy transactions
	AccessList *types2.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big       `json:"chainId,omitempty"`

	// Signature values
	V *hexutil.Big `json:"v" gencodec:"required"`
	R *hexutil.Big `json:"r" gencodec:"required"`
	S *hexutil.Big `json:"s" gencodec:"required"`
}

type Header struct {
	ParentHash  *libcommon.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash   *libcommon.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    *libcommon.Address `json:"miner"            gencodec:"required"`
	Root        *libcommon.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      *libcommon.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash *libcommon.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom       *types.Bloom       `json:"logsBloom"        gencodec:"required"`
	Difficulty  *hexutil.Big       `json:"difficulty"       gencodec:"required"`
	Number      *hexutil.Big       `json:"number"           gencodec:"required"`
	GasLimit    *hexutil.Uint64    `json:"gasLimit"         gencodec:"required"`
	GasUsed     *hexutil.Uint64    `json:"gasUsed"          gencodec:"required"`
	Time        *hexutil.Uint64    `json:"timestamp"        gencodec:"required"`
	Extra       *hexutil.Bytes     `json:"extraData"        gencodec:"required"`
	MixDigest   *libcommon.Hash    `json:"mixHash"`
	Nonce       *types.BlockNonce  `json:"nonce"`
	BaseFee     *hexutil.Big       `json:"baseFeePerGas" rlp:"optional"`
	// transactions
	Transactions []SendTxArgs `json:"transactions"`
}

func (dec *Header) ToHeader() types.Header {
	var h types.Header
	h.ParentHash = *dec.ParentHash
	h.UncleHash = *dec.UncleHash
	h.Coinbase = *dec.Coinbase
	h.Root = *dec.Root
	h.TxHash = *dec.TxHash
	h.ReceiptHash = *dec.ReceiptHash
	h.Bloom = *dec.Bloom
	h.Difficulty = (*big.Int)(dec.Difficulty)
	h.Number = (*big.Int)(dec.Number)
	h.GasLimit = uint64(*dec.GasLimit)
	h.GasUsed = uint64(*dec.GasUsed)
	h.Time = uint64(*dec.Time)
	h.Extra = *dec.Extra
	if dec.MixDigest != nil {
		h.MixDigest = *dec.MixDigest
	}
	if dec.Nonce != nil {
		h.Nonce = *dec.Nonce
	}
	if dec.BaseFee != nil {
		h.BaseFee = (*big.Int)(dec.BaseFee)
	}
	return h
}

// ToTransaction converts the arguments to a transaction.
func (args *SendTxArgs) ToTransaction() *types.Transaction {
	// Add the To-field, if specified
	var to *libcommon.Address
	if args.To != nil {
		dstAddr := args.To.Address()
		to = &dstAddr
	}

	var input []byte
	if args.Input != nil {
		input = *args.Input
	} else if args.Data != nil {
		input = *args.Data
	}

	var data types.Transaction

	chainId, _ := uint256.FromBig((*big.Int)(args.ChainID))
	value, _ := uint256.FromBig((*big.Int)(&args.Value))
	gasPrice, _ := uint256.FromBig((*big.Int)(args.GasPrice))
	tip, _ := uint256.FromBig((*big.Int)(args.MaxPriorityFeePerGas))
	feeCap, _ := uint256.FromBig((*big.Int)(args.MaxFeePerGas))
	V, _ := uint256.FromBig((*big.Int)(args.V))
	R, _ := uint256.FromBig((*big.Int)(args.R))
	S, _ := uint256.FromBig((*big.Int)(args.S))

	switch {
	case args.MaxFeePerGas != nil:
		al := types2.AccessList{}
		if args.AccessList != nil {
			al = *args.AccessList
		}
		data = &types.DynamicFeeTransaction{
			CommonTx: types.CommonTx{
				ChainID: chainId,
				To:      to,
				Nonce:   uint64(args.Nonce),
				Gas:     uint64(args.Gas),
				Value:   value,
				Data:    input,
				V:       *V,
				R:       *R,
				S:       *S,
			},
			AccessList: al,
			Tip:        tip,
			FeeCap:     feeCap,
		}
	case args.AccessList != nil:
		data = &types.AccessListTx{
			LegacyTx: types.LegacyTx{
				CommonTx: types.CommonTx{
					ChainID: chainId,
					To:      to,
					Nonce:   uint64(args.Nonce),
					Gas:     uint64(args.Gas),
					Value:   value,
					Data:    input,
					V:       *V,
					R:       *R,
					S:       *S,
				},
				GasPrice: gasPrice,
			},
			ChainID:    chainId,
			AccessList: *args.AccessList,
		}
	default:
		data = &types.LegacyTx{
			CommonTx: types.CommonTx{
				ChainID: chainId,
				To:      to,
				Nonce:   uint64(args.Nonce),
				Gas:     uint64(args.Gas),
				Value:   value,
				Data:    input,
				V:       *V,
				R:       *R,
				S:       *S,
			},
			GasPrice: gasPrice,
		}
	}
	return &data
}

//go:build !mips
// +build !mips

package oracle

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"

	goethereumhex "github.com/ethereum/go-ethereum/common/hexutil"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/rlp"
)

type jsonreq struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      uint64        `json:"id"`
}

type jsonresp struct {
	Jsonrpc string        `json:"jsonrpc"`
	Id      uint64        `json:"id"`
	Result  AccountResult `json:"result"`
}

type jsonresps struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      uint64 `json:"id"`
	Result  string `json:"result"`
}

type jsonrespi struct {
	Jsonrpc string         `json:"jsonrpc"`
	Id      uint64         `json:"id"`
	Result  hexutil.Uint64 `json:"result"`
}

type jsonrespt struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      uint64 `json:"id"`
	Result  Header `json:"result"`
}

// Result structs for GetProof
type AccountResult struct {
	Address      libcommon.Address `json:"address"`
	AccountProof []string          `json:"accountProof"`
	Balance      *hexutil.Big      `json:"balance"`
	CodeHash     libcommon.Hash    `json:"codeHash"`
	Nonce        hexutil.Uint64    `json:"nonce"`
	StorageHash  libcommon.Hash    `json:"storageHash"`
	StorageProof []StorageResult   `json:"storageProof"`
}

type StorageResult struct {
	Key   string       `json:"key"`
	Value *hexutil.Big `json:"value"`
	Proof []string     `json:"proof"`
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     libcommon.Hash // merkle root of the storage trie
	CodeHash []byte
}

var nodeUrl = "https://mainnet.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161"

func SetNodeUrl(newNodeUrl string) {
	nodeUrl = newNodeUrl
}

func toFilename(key string) string {
	return fmt.Sprintf("%s/json_%s", root, key)
}

func cacheRead(key string) []byte {
	dat, err := ioutil.ReadFile(toFilename(key))
	if err == nil {
		return dat
	}
	panic("cache missing")
}

func cacheExists(key string) bool {
	_, err := os.Stat(toFilename(key))
	return err == nil
}

func cacheWrite(key string, value []byte) {
	ioutil.WriteFile(toFilename(key), value, 0644)
}

func getAPI(jsonData []byte) io.Reader {
	key := goethereumhex.Encode(crypto.Keccak256(jsonData))
	if cacheExists(key) {
		return bytes.NewReader(cacheRead(key))
	}
	resp, _ := http.Post(nodeUrl, "application/json", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()
	ret, _ := ioutil.ReadAll(resp.Body)
	cacheWrite(key, ret)
	return bytes.NewReader(ret)
}

var unhashMap = make(map[libcommon.Hash]libcommon.Address)

func unhash(addrHash libcommon.Hash) libcommon.Address {
	return unhashMap[addrHash]
}

var cached = make(map[string]bool)

func PrefetchStorage(blockNumber *big.Int, addr libcommon.Address, skey libcommon.Hash, postProcess func(map[libcommon.Hash][]byte)) {
	key := fmt.Sprintf("proof_%d_%s_%s", blockNumber, addr, skey)
	if cached[key] {
		return
	}
	cached[key] = true

	ap := getProofAccount(blockNumber, addr, skey, true)
	//fmt.Println("PrefetchStorage", blockNumber, addr, skey, len(ap))
	newPreimages := make(map[libcommon.Hash][]byte)
	for _, s := range ap {
		ret, _ := hex.DecodeString(s[2:])
		hash := crypto.Keccak256Hash(ret)
		//fmt.Println("   ", i, hash)
		newPreimages[hash] = ret
	}

	if postProcess != nil {
		postProcess(newPreimages)
	}

	for hash, val := range newPreimages {
		preimages[hash] = val
	}
}

func PrefetchAccount(blockNumber *big.Int, addr libcommon.Address, postProcess func(map[libcommon.Hash][]byte)) {
	key := fmt.Sprintf("proof_%d_%s", blockNumber, addr)
	if cached[key] {
		return
	}
	cached[key] = true

	ap := getProofAccount(blockNumber, addr, libcommon.Hash{}, false)
	newPreimages := make(map[libcommon.Hash][]byte)
	for _, s := range ap {
		ret, _ := hex.DecodeString(s[2:])
		hash := crypto.Keccak256Hash(ret)
		newPreimages[hash] = ret
	}

	if postProcess != nil {
		postProcess(newPreimages)
	}

	for hash, val := range newPreimages {
		preimages[hash] = val
	}
}

func PrefetchCode(blockNumber *big.Int, addrHash libcommon.Hash) {
	key := fmt.Sprintf("code_%d_%s", blockNumber, addrHash)
	if cached[key] {
		return
	}
	cached[key] = true
	ret := getProvedCodeBytes(blockNumber, addrHash)
	hash := crypto.Keccak256Hash(ret)
	preimages[hash] = ret
}

var inputhash libcommon.Hash

func InputHash() libcommon.Hash {
	return inputhash
}

var inputs [6]libcommon.Hash
var outputs [2]libcommon.Hash

func Output(output libcommon.Hash, receipts libcommon.Hash) {
	if receipts != outputs[1] {
		fmt.Println("WARNING, receipts don't match", receipts, "!=", outputs[1])
		panic("BAD receipts")
	}
	if output == outputs[0] {
		fmt.Println("good transition")
	} else {
		fmt.Println(output, "!=", outputs[0])
		panic("BAD transition :((")
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func prefetchUncles(blockHash libcommon.Hash, uncleHash libcommon.Hash) {
	jr := jsonrespi{}
	{
		r := jsonreq{Jsonrpc: "2.0", Method: "eth_getUncleCountByBlockHash", Id: 1}
		r.Params = make([]interface{}, 1)
		r.Params[0] = blockHash.Hex()
		jsonData, _ := json.Marshal(r)
		check(json.NewDecoder(getAPI(jsonData)).Decode(&jr))
	}

	var uncles []*types.Header
	for u := 0; u < int(jr.Result); u++ {
		jr2 := jsonrespt{}
		{
			r := jsonreq{Jsonrpc: "2.0", Method: "eth_getUncleByBlockHashAndIndex", Id: 1}
			r.Params = make([]interface{}, 2)
			r.Params[0] = blockHash.Hex()
			r.Params[1] = fmt.Sprintf("0x%x", u)
			jsonData, _ := json.Marshal(r)

			/*a, _ := ioutil.ReadAll(getAPI(jsonData))
			fmt.Println(string(a))*/

			check(json.NewDecoder(getAPI(jsonData)).Decode(&jr2))
		}
		uncleHeader := jr2.Result.ToHeader()
		uncles = append(uncles, &uncleHeader)
		//fmt.Println(uncleHeader)
		//fmt.Println(jr2.Result)
	}

	unclesRlp, _ := rlp.EncodeToBytes(uncles)
	hash := crypto.Keccak256Hash(unclesRlp)

	if hash != uncleHash {
		panic("wrong uncle hash")
	}

	preimages[hash] = unclesRlp
}

func PrefetchBlock(blockNumber *big.Int, startBlock bool) {
	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getBlockByNumber", Id: 1}
	r.Params = make([]interface{}, 2)
	r.Params[0] = fmt.Sprintf("0x%x", blockNumber.Int64())
	r.Params[1] = true
	jsonData, err := json.Marshal(r)
	check(err)

	/*dat, _ := ioutil.ReadAll(getAPI(jsonData))
	fmt.Println(string(dat))*/

	jr := jsonrespt{}
	check(json.NewDecoder(getAPI(jsonData)).Decode(&jr))
	//fmt.Println(jr.Result)
	blockHeader := jr.Result.ToHeader()

	// put in the start block header
	if startBlock {
		blockHeaderRlp, err := rlp.EncodeToBytes(&blockHeader)
		check(err)
		hash := crypto.Keccak256Hash(blockHeaderRlp)
		preimages[hash] = blockHeaderRlp
		emptyHash := libcommon.Hash{}
		if inputs[0] == emptyHash {
			inputs[0] = hash
		}
		return
	}

	// second block
	if blockHeader.ParentHash != inputs[0] {
		fmt.Println(blockHeader.ParentHash, inputs[0])
		panic("block transition isn't correct")
	}
	inputs[1] = blockHeader.TxHash
	inputs[2] = blockHeader.Coinbase.Hash()
	inputs[3] = blockHeader.UncleHash
	inputs[4] = libcommon.BigToHash(big.NewInt(int64(blockHeader.GasLimit)))
	inputs[5] = libcommon.BigToHash(big.NewInt(int64(blockHeader.Time)))

	// save the inputs
	saveinput := make([]byte, 0)
	for i := 0; i < len(inputs); i++ {
		saveinput = append(saveinput, inputs[i].Bytes()[:]...)
	}
	inputhash = crypto.Keccak256Hash(saveinput)
	preimages[inputhash] = saveinput
	ioutil.WriteFile(fmt.Sprintf("%s/input", root), inputhash.Bytes(), 0644)
	//ioutil.WriteFile(fmt.Sprintf("%s/input", root), saveinput, 0644)

	// secret input aka output
	outputs[0] = blockHeader.Root
	outputs[1] = blockHeader.ReceiptHash

	// save the outputs
	saveoutput := make([]byte, 0)
	for i := 0; i < len(outputs); i++ {
		saveoutput = append(saveoutput, outputs[i].Bytes()[:]...)
	}
	ioutil.WriteFile(fmt.Sprintf("%s/output", root), saveoutput, 0644)

	// save the txs
	txs := make([]types.Transaction, len(jr.Result.Transactions))
	for i := 0; i < len(jr.Result.Transactions); i++ {
		txs[i] = *(jr.Result.Transactions[i].ToTransaction())
	}
	testTxHash := types.DeriveSha(types.Transactions(txs))
	if testTxHash != blockHeader.TxHash {
		fmt.Println(testTxHash, "!=", blockHeader.TxHash)
		panic("tx hash derived wrong")
	}

	// save the uncles
	prefetchUncles(blockHeader.Hash(), blockHeader.UncleHash)
}

func getProofAccount(blockNumber *big.Int, addr libcommon.Address, skey libcommon.Hash, storage bool) []string {
	addrHash := crypto.Keccak256Hash(addr[:])
	unhashMap[addrHash] = addr

	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getProof", Id: 1}
	r.Params = make([]interface{}, 3)
	r.Params[0] = addr
	r.Params[1] = [1]libcommon.Hash{skey}
	r.Params[2] = fmt.Sprintf("0x%x", blockNumber.Int64())
	jsonData, _ := json.Marshal(r)
	jr := jsonresp{}
	json.NewDecoder(getAPI(jsonData)).Decode(&jr)

	if storage {
		return jr.Result.StorageProof[0].Proof
	} else {
		return jr.Result.AccountProof
	}
}

func getProvedCodeBytes(blockNumber *big.Int, addrHash libcommon.Hash) []byte {
	addr := unhash(addrHash)

	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getCode", Id: 1}
	r.Params = make([]interface{}, 2)
	r.Params[0] = addr
	r.Params[1] = fmt.Sprintf("0x%x", blockNumber.Int64())
	jsonData, _ := json.Marshal(r)
	jr := jsonresps{}
	json.NewDecoder(getAPI(jsonData)).Decode(&jr)

	//fmt.Println(jr.Result)

	// curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b", "0x2"],"id":1}'

	ret, _ := hex.DecodeString(jr.Result[2:])
	//fmt.Println(ret)
	return ret
}

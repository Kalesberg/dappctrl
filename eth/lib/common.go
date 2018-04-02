package lib

import (
	"strconv"
	"strings"
)

// This module provides low-level methods for accessing common ethereum info.
// For detailed API description, please refer to:
// https://ethereumbuilders.gitbooks.io/guide/content/en/ethereum_json_rpc.html

type GasPriceAPIResponse struct {
	apiResponse
	Result string `json:"result"`
}

// GetGasPrice returns current gas price in wei.
// For the details, please, refer to:
// https://ethereumbuilders.gitbooks.io/guide/content/en/ethereum_json_rpc.html#eth_gasprice
func (e *EthereumClient) GetGasPrice() (*GasPriceAPIResponse, error) {
	response := &GasPriceAPIResponse{}
	return response, e.fetch("eth_gasPrice", "", response)
}

//---------------------------------------------------------------------------------------------------------------------

type BlockNumberAPIResponse struct {
	GasPriceAPIResponse
	BlockNumber uint64
}

// GetBlockNumber returns the number of most recent block in blockchain.
// For the details, please, refer to:
// https://ethereumbuilders.gitbooks.io/guide/content/en/ethereum_json_rpc.html#eth_blocknumber
func (e *EthereumClient) GetBlockNumber() (*BlockNumberAPIResponse, error) {
	response := &BlockNumberAPIResponse{}
	err := e.fetch("eth_blockNumber", "", response)
	if err != nil {
		return nil, err
	}

	// todo: add/refactor tests for block number tests
	response.BlockNumber, err = strconv.ParseUint(response.Result[2:], 16, 64)
	return response, err
}

//---------------------------------------------------------------------------------------------------------------------

type logInfo struct {
	LogIndex         string   `json:"logIndex"`
	TransactionIndex string   `json:"transactionIndex"`
	TransactionHash  string   `json:"transactionHash"`
	BlockHash        string   `json:"blockHash"`
	BlockNumber      string   `json:"blockNumber"`
	Address          string   `json:"address"`
	Data             string   `json:"data"`
	Topics           []string `json:"topics"`
	Type             string   `json:"type"`
}

type transactionInfo struct {
	TransactionHash   string     `json:"transactionHash"`
	TransactionIndex  string     `json:"transactionIndex"`
	BlockHash         string     `json:"blockHash"`
	BlockNumber       string     `json:"blockNumber"`
	GasUsed           string     `json:"gasUsed"`
	CumulativeGasUsed string     `json:"cumulativeGasUsed"`
	ContractAddress   string     `json:"contractAddress"`
	Logs              []*logInfo `json:"logs"`
	Status            string     `json:"status"`
	LogsBloom         string     `json:"logsBloom"`
}

type TransactionReceiptAPIResponse struct {
	apiResponse
	Result transactionInfo `json:"result"`
}

// GetTransactionReceipt returns receipt of the transactionInfo,
// specified by the hash.
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_gettransactionreceipt
func (e *EthereumClient) GetTransactionReceipt(hash string) (*TransactionReceiptAPIResponse, error) {
	if len(hash) > 2 {
		if strings.ToLower(hash[:2]) != "0x" {
			hash = "0x" + hash
		}
	}

	response := &TransactionReceiptAPIResponse{}
	return response, e.fetch("eth_getTransactionReceipt", `"`+hash+`"`, response)
}

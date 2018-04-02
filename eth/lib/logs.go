package lib

// This module provides low-level methods for accessing ethereum logs.
// For detailed API description, please refer to:
// https://ethereumbuilders.gitbooks.io/guide/content/en/ethereum_json_rpc.html

import (
	"encoding/json"
	"errors"
	"fmt"
)

type LogsAPIRecord struct {
	Type                string   `json:"type"`
	TransactionIndexHex string   `json:"transactionIndex"`
	LogIndexHex         string   `json:"logIndex"`
	TransactionHash     string   `json:"transactionHash"`
	Address             string   `json:"address"`
	BlockHash           string   `json:"blockHash"`
	Data                string   `json:"data"`
	Topics              []string `json:"topics"`
	BlockNumberHex      string   `json:"blockNumber"`
}

type LogsAPIResponse struct {
	apiResponse
	Result []LogsAPIRecord `json:"result"`
}

// GetLog fetches logs form remote geth node.
//
// "topics" contains topics, that must be used for filtering.
// "fromBlock" - specifies first block number **from** which lookup must be performed.
// "toBlock" - specifies last block number **to** which lookup must be performed.
//
// Tests: logs_test/TestNormalLogsFetching
// Tests: logs_test/TestNegativeLogsFetching
// Tests: logs_test/TestLogsFetchingWithBrokenNetwork
func (e *EthereumClient) GetLogs(contractAddress string, topics []string, fromBlock, toBlock string) (*LogsAPIResponse, error) {
	if contractAddress == "" {
		return nil, errors.New("contract address is required")
	}

	if fromBlock == "" {
		fromBlock = "earliest"
	} else {
		if len(fromBlock) < 2 || (fromBlock != "earliest" && fromBlock[:2] != "0x") {
			return nil, errors.New("fromBlock must be in hex format")
		}
	}

	if toBlock == "" {
		toBlock = "latest"
	} else {
		if len(toBlock) < 2 || (toBlock != "latest" && toBlock[:2] != "0x") {
			return nil, errors.New("toBlock must be in hex format")
		}
	}

	// Note: topics are not checked for emptiness,
	// because empty topics are allowed by the geth-API:
	// in this case all events of the contract would be returned.
	for _, topic := range topics {
		if len(topic) > 2 && topic[:2] != "0x" {
			topic = "0x" + topic
		}

		const kTopicLength = 2 + 64 // "0x" + 64 symbols (256 bits in hex).
		if len(topic) != kTopicLength {
			return nil, errors.New("invalid topic occurred: " + topic)
		}
	}

	topicsJson, err := json.Marshal(topics)
	if err != nil {
		return nil, errors.New("can't marshall topics: " + err.Error())
	}

	params := fmt.Sprintf(`{"topics":%s,"address":"%s","fromBlock":"%s","toBlock":"%s"}`,
		topicsJson, contractAddress, fromBlock, toBlock)

	response := &LogsAPIResponse{}
	err = e.fetch("eth_getLogs", params, response)
	if err != nil {
		return nil, errors.New("can't fetch response: " + err.Error())
	}

	return response, nil
}

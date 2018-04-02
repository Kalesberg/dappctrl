package bcmon

import "dappctrl/util"

// EthNode specifies host and port of the remote ethereum node.
type EthNode struct {
	util.NetworkInterface
}

type Config struct {
	// Specifies interface of remote ethereum node,
	// that would be used a gateway into the eth. network.
	EthNode EthNode `json:"EthNode"`

	// Specifies how many blocks from chain tail must be ignored,
	// before fetching them for the processing.
	//
	// WARN: This parameter is security-critical!
	// In case if it would be set to relatively small value -
	// probability of processing of invalid events would increase significantly.
	ChallengeBlocksCount uint16 `json:"ChallengeBlocksCount"`

	ContractAddress string `json:"ContractAddress"`
}

func NewDefaultConfig() *Config {
	return &Config{
		EthNode: EthNode{
			util.NetworkInterface{"127.0.0.1", 8545},
		},
		ChallengeBlocksCount: 6,
	}
}

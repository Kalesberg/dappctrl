package bcmon

import (
	"dappctrl/eth/lib"
	"dappctrl/util"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"gopkg.in/reform.v1"
	"time"
)

const (
	kRunning  = 1
	kStopping = 2
	kStopped  = 3
)

type BlockchainMonitor struct {
	client *lib.EthereumClient

	// Specifies number of block, from which events was loaded last time.
	lastFetchedBlockNumber uint64

	conf   *Config
	logger *util.Logger
	db     *reform.DB

	state int
}

func NewMonitor(conf *Config, logger *util.Logger, db *reform.DB) (*BlockchainMonitor, error) {
	if conf == nil {
		return nil, errors.New("invalid, or empty settings occurred")
	}

	monitor := &BlockchainMonitor{
		client: lib.NewEthereumClient(conf.EthNode.Host, conf.EthNode.Port),
		conf:   conf,
		logger: logger,
		db:     db,

		state: kRunning,
	}

	err := monitor.restoreLastFetchedBlockNumber(db)
	if err != nil {
		return nil, err
	}

	return monitor, nil
}

func (bm *BlockchainMonitor) MonitorEvents() error {
	for {
		// This method might be spawned in separate goroutine.
		// If Close() was called - then this method must be stopped.
		// This is achieved by controlling special state flag.
		if bm.state == kStopping {
			bm.state = kStopped
			return nil
		}

		if err := bm.processRound(); err != nil {
			return err
		}
		time.Sleep(bm.delay())
	}
}

// todo: add comments
func (bm *BlockchainMonitor) Close() {
	bm.state = kStopping

	// Block until separate goroutine would not report correct closing.
	for {
		if bm.state == kStopped {
			return
		}
		time.Sleep(time.Second)
	}
}

// delay returns time duration between two blockchain requests.
func (bm *BlockchainMonitor) delay() time.Duration {
	return time.Second * 20
}

// todo: add comments
func (bm *BlockchainMonitor) processRound() error {
	blockFrom, blockTo, err := bm.calculateNextBlocksRange()
	if err != nil {
		return err
	}

	if (blockTo - blockFrom) <= 0 {
		// No new blocks occurred in the chain.
		// No need to try to receive any events.
		return nil
	}

	logs, err := bm.fetchEvents(blockFrom, blockTo)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		// There is no new events received.
		// No need to start database transaction.
		return nil
	}

	tx, err := bm.db.Begin()
	if err != nil {
		return err
	}

	for _, event := range logs {
		txInfoResponse, err := bm.client.GetTransactionReceipt(event.TransactionHash)
		if err != nil {
			tx.Rollback()
			return err
		}

		if txInfoResponse.Result.Status != "0x01" {
			bm.logger.Warn("",
				"Unconfirmed ethereum transaction occurred. "+
					"Events related to it would be ignored. "+
					"Transaction details: ", txInfoResponse, "\n\n",
				"Events details: ", event)
			continue
		}

		// todo: debug
		fmt.Println(txInfoResponse)

		// todo: alter scheme (alter table eth_logs add column log_index_hex text not null;)

		var eventAlreadyProcessed = false
		if eventAlreadyProcessed, err = bm.checkEventWasProcessedInPast(event, tx); err != nil {
			tx.Rollback()
			return err
		}

		if eventAlreadyProcessed {
			bm.logger.Warn("",
				"Duplicated event occurred. "+
					"Transaction details: ", txInfoResponse,
				"Events details: ", event)
			continue
		}

		if err := bm.writeEventIntoDB(event, txInfoResponse, tx); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// todo: add comments
func (bm *BlockchainMonitor) calculateNextBlocksRange() (uint64, uint64, error) {
	response, err := bm.client.GetBlockNumber()
	if err != nil {
		return 0, 0, err
	}

	blockTo := response.BlockNumber - uint64(bm.conf.ChallengeBlocksCount)
	blockFrom := bm.lastFetchedBlockNumber
	return blockFrom, blockTo, nil
}

// fetchEvents requests all possible types of events from the remote geth node,
// packs them into common slice and returns.
// Due to the geth API enforcements - several separate requests must be done,
// to fetch all possible events.
func (bm *BlockchainMonitor) fetchEvents(blockFrom, blockTo uint64) ([]*lib.LogsAPIRecord, error) {
	var kEventDigests = []string{
		lib.EthChannelCreated,
		lib.EthChannelToppedUp,
		lib.EthChannelCloseRequested,
		lib.EthOfferingCreated,
		lib.EthOfferingDeleted,
		lib.EthServiceOfferingEndpoint,
		lib.EthServiceOfferingSupplyChanged,
		lib.EthServiceOfferingPoppedUp,
		lib.EthCooperativeChannelClose,
		lib.EthUncooperativeChannelClose,
	}

	receivedEvents := make([]*lib.LogsAPIRecord, 0, 256)
	for _, digest := range kEventDigests {
		response, err := bm.client.GetLogs(
			bm.conf.ContractAddress,
			[]string{"0x" + digest},
			fmt.Sprintf("0x%x", blockFrom),
			fmt.Sprintf("0x%x", blockTo))

		if err != nil {
			return nil, err
		}

		for _, logReceived := range response.Result {
			receivedEvents = append(receivedEvents, &logReceived)
		}
	}

	return receivedEvents, nil
}

// registerRelatedJob creates record about enqueued job, related to the "event".
// Related job should process inform the application about newly occurred event,
// and apply corresponding logic.
func (bm *BlockchainMonitor) registerRelatedJob(event *lib.LogsAPIRecord, tx *reform.TX) error {
	// todo: add filtering by event client/agent address and offering hash

	// todo: [blocked by simon] add implementation.
	return nil
}

func (bm *BlockchainMonitor) writeEventIntoDB(
	event *lib.LogsAPIRecord, eventTX *lib.TransactionReceiptAPIResponse, tx *reform.TX) error {

	newEventUUID, err := uuid.NewV4()
	if err != nil {
		return err
	}

	eventTopics, err := json.Marshal(event.Topics)
	if err != nil {
		return err
	}

	err = bm.registerRelatedJob(event, tx)
	if err != nil {
		return err
	}

	blockNumber, err := lib.NewUint256(event.BlockNumberHex)
	if err != nil {
		return err
	}

	logIndex, err := lib.NewUint256(event.LogIndexHex)
	if err != nil {
		return err
	}

	transactionHash, err := hex.DecodeString(event.TransactionHash[2:]) // dropping "0x"
	if err != nil {
		return err
	}

	address, err := hex.DecodeString(event.Address[2:]) // dropping "0x"
	if err != nil {
		return err
	}
	println("\n\n\n", event.Address, "\n\n\n")

	query := "INSERT INTO eth_logs " +
		"(id, tx_hash, status, job, block_number, log_index, addr, data, topics) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"

	_, err = tx.Exec(query,
		newEventUUID.String(),
		base64.StdEncoding.EncodeToString(transactionHash),
		"mined",
		nil, // todo: add id of related job here
		blockNumber.ToBigInt().String(),
		logIndex.ToBigInt().String(),
		base64.StdEncoding.EncodeToString(address),
		event.Data,
		eventTopics)

	return err
}

// todo: add comments
func (bm *BlockchainMonitor) checkEventWasProcessedInPast(event *lib.LogsAPIRecord, tx *reform.TX) (bool, error) {
	eventAlreadyProcessed := false

	logIndex, err := lib.NewUint256(event.LogIndexHex)
	if err != nil {
		return false, err
	}

	transactionHash, err := hex.DecodeString(event.TransactionHash[2:]) // dropping "0x"
	if err != nil {
		return false, err
	}

	query := fmt.Sprintf(
		`SELECT count(*) >= 1 FROM eth_logs WHERE tx_hash = '%s' AND log_index = '%s'`,
		base64.StdEncoding.EncodeToString(transactionHash),
		logIndex.ToBigInt().String())

	err = tx.QueryRow(query).Scan(&eventAlreadyProcessed)
	if err != nil {
		return false, err
	}

	return eventAlreadyProcessed, nil
}

// todo: add comments
func (bm *BlockchainMonitor) restoreLastFetchedBlockNumber(db *reform.DB) error {
	var lastFetchedBlockNumber uint64 = 0
	err := db.QueryRow("SELECT max(block_number) FROM eth_logs").Scan(&lastFetchedBlockNumber)
	if err != nil {
		return err
	}

	bm.lastFetchedBlockNumber = lastFetchedBlockNumber
	return nil
}

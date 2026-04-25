package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/GPadaka19/attesta-be/config"
	"github.com/GPadaka19/attesta-be/model"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	monadChainID      = 10143
	chainTimeout      = 60 * time.Second
	monadExplorerBase = "https://testnet.monadexplorer.com/tx/"
)

type attestationCreatedEvent struct {
	ID         *big.Int       `json:"id"`
	Attester   common.Address `json:"attester"`
	Recipient  common.Address `json:"recipient"`
	Repo       string         `json:"repo"`
	IsVerified bool           `json:"isVerified"`
}

type AttestResult struct {
	AttestationID uint64
	TxHash        string
	ExplorerURL   string
}

func SubmitAttestation(owner, repo, author string, recipient common.Address, proof *model.SkillProof, commitsAnalyzed int, evidenceCID string) (*AttestResult, error) {
	if config.App.ContractAddress == "" {
		return nil, fmt.Errorf("CONTRACT_NOT_SET")
	}
	if config.App.DeployerKey == "" {
		return nil, fmt.Errorf("TX_FAILED: DEPLOYER_PRIVATE_KEY is empty")
	}

	abiJSON, err := os.ReadFile("abi/Attesta.json")
	if err != nil {
		return nil, fmt.Errorf("chain: read abi/Attesta.json: %w", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(abiJSON)))
	if err != nil {
		return nil, fmt.Errorf("chain: parse ABI: %w", err)
	}

	rpc := config.App.MonadRPC
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return nil, fmt.Errorf("TX_FAILED: dial rpc: %w", err)
	}
	defer client.Close()

	privHex := strings.TrimPrefix(config.App.DeployerKey, "0x")
	priv, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return nil, fmt.Errorf("TX_FAILED: invalid deployer key: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(priv, big.NewInt(monadChainID))
	if err != nil {
		return nil, fmt.Errorf("TX_FAILED: create transactor: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), chainTimeout)
	defer cancel()
	auth.Context = ctx
	auth.From = crypto.PubkeyToAddress(priv.PublicKey)

	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err == nil {
		auth.Nonce = big.NewInt(int64(nonce))
	}

	if gp, gErr := client.SuggestGasPrice(ctx); gErr == nil {
		auth.GasPrice = gp
	}

	contractAddr := common.HexToAddress(config.App.ContractAddress)
	contract := bind.NewBoundContract(contractAddr, parsedABI, client, client, client)

	contributionType := strings.Join(proof.ContributionTypes, ", ")
	if contributionType == "" {
		contributionType = "unknown"
	}

	if commitsAnalyzed < 0 {
		commitsAnalyzed = 0
	}
	commitCount := new(big.Int).SetUint64(uint64(commitsAnalyzed))

	conf := proof.ConfidenceScore
	if conf < 0 {
		conf = 0
	}
	if conf > 100 {
		conf = 100
	}

	log.Printf("[Chain] submitting attest recipient=%s repo=%s commits=%d", recipient.Hex(), owner+"/"+repo, commitsAnalyzed)
	tx, err := contract.Transact(
		auth,
		"attest",
		recipient,
		author,
		owner+"/"+repo,
		proof.PrimaryLanguage,
		contributionType,
		proof.I18nOnly,
		proof.SkillTags,
		uint8(conf),
		commitCount,
		evidenceCID,
	)
	if err != nil {
		log.Printf("[Chain] transact error: %v", err)
		return nil, fmt.Errorf("TX_FAILED: transact: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		log.Printf("[Chain] wait mined error tx=%s: %v", tx.Hash().Hex(), err)
		return nil, fmt.Errorf("TX_FAILED: wait mined: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		log.Printf("[Chain] receipt failed tx=%s status=%d", tx.Hash().Hex(), receipt.Status)
		return nil, fmt.Errorf("TX_FAILED: receipt status %d", receipt.Status)
	}

	id, err := parseAttestationCreatedID(parsedABI, receipt.Logs)
	if err != nil {
		log.Printf("[Chain] parse event error tx=%s: %v", tx.Hash().Hex(), err)
		return nil, fmt.Errorf("TX_FAILED: parse event: %w", err)
	}

	return &AttestResult{
		AttestationID: id,
		TxHash:        tx.Hash().Hex(),
		ExplorerURL:   monadExplorerBase + tx.Hash().Hex(),
	}, nil
}

func parseAttestationCreatedID(contractABI abi.ABI, logs []*types.Log) (uint64, error) {
	ev, ok := contractABI.Events["AttestationCreated"]
	if !ok {
		return 0, fmt.Errorf("chain: event AttestationCreated not in ABI")
	}

	for _, lg := range logs {
		if len(lg.Topics) == 0 || lg.Topics[0] != ev.ID {
			continue
		}

		// `id` is indexed uint256, so it's encoded directly in topic[1] (32-byte big-endian).
		if len(lg.Topics) < 2 {
			return 0, fmt.Errorf("chain: event id topic missing")
		}
		id := new(big.Int).SetBytes(lg.Topics[1].Bytes())
		return id.Uint64(), nil
	}

	return 0, fmt.Errorf("chain: AttestationCreated not found in receipt")
}

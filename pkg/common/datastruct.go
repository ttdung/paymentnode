package common

import (
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type Channel_st struct {
	Index           string
	Multisig_Addr   string
	Multisig_Pubkey cryptoTypes.PubKey
	PartA           string
	PartB           string
	PubkeyA         cryptoTypes.PubKey
	PubkeyB         cryptoTypes.PubKey
	Denom           string
	Amount_partA    float64
	Amount_partB    float64
	Timelock        uint64
}

type Commitment_st struct {
	ChannelID          string
	Denom              string
	BalanceA           float64
	BalanceB           float64
	HashcodeA          string
	HashcodeB          string
	SecretA            string
	SecretB            string
	StrSigA            string
	StrSigB            string
	TxByteForBroadcast []byte
	PenaltyA_Tx        string // if this commitment is invalidated, broadcast this to fire the cheating peer in case.
	PenaltyB_Tx        string
	Timelock           uint64
	Nonce              uint64
}

const (
	COINTYPE = uint32(1) // test type for this project
	TIMELOCK = uint32(3)
)

//const COINTYPE = ethermintTypes.Bip44CoinType

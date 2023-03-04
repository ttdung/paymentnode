package utils

import (
	"fmt"
	channelTypes "github.com/AstraProtocol/channel/x/channel/types"
	"github.com/cosmos/cosmos-sdk/client"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/astra-go-sdk/channel"
	sdkcommon "github.com/dungtt-astra/astra-go-sdk/common"
	"github.com/dungtt-astra/paymentnode/pkg/common"
)

func BuildCommitmentMsg(com *common.Commitment_st, chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {

	msg := channelTypes.MsgCommitment{
		Creator: chann.Multisig_Addr,
		From:    chann.PartA,
		CoinToCreator: &sdk.Coin{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceA)),
		},
		ToTimelockAddr: chann.PartB, //peer node
		Timelock:       com.Timelock,
		ToHashlockAddr: chann.PartA,
		Hashcode:       com.HashcodeB,
		CoinToHtlc: &sdk.Coin{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceB)),
		},
		ChannelID: chann.Index,
	}

	openChannelRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return openChannelRequest
}
func BuildAndSignCommitmentMsg(rpcClient client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildCommitmentMsg(com, chann, 200000, "25aastra")

	fmt.Println("Multisig Address: ", chann.Multisig_Addr)
	fmt.Println("openChannelRequest: ", openChannelRequest)

	strSig, err := channel.NewChannel(rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}

	return openChannelRequest, strSig, nil
}

func BuildOpenChannelMsg(chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {
	msg := channelTypes.MsgOpenChannel{
		Creator: chann.Multisig_Addr,
		PartA:   chann.PartA,
		PartB:   chann.PartB,
		CoinA: &sdk.Coin{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(chann.Amount_partA)),
		},
		CoinB: &sdk.Coin{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(chann.Amount_partB)),
		},
		MultisigAddr: chann.Multisig_Addr,
	}

	openChannelRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return openChannelRequest
}

func BuildAndSignOpenChannelMsg(rpcClient client.Context, account *account.PrivateKeySerialized, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildOpenChannelMsg(chann, 200000, "25aastra")
	//fmt.Println("openChannelRequest:", openChannelRequest)

	strSig, err := channel.NewChannel(rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}

	return openChannelRequest, strSig, nil
}

func BuildAndBroadCastMultisigMsg(client client.Context, multiSigPubkey cryptoTypes.PubKey, sig1, sig2 string, msgRequest channel.SignMsgRequest) (*sdk.TxResponse, error) {

	signList := make([][]signingTypes.SignatureV2, 0)

	signByte1, err := sdkcommon.TxBuilderSignatureJsonDecoder(client.TxConfig, sig1)
	if err != nil {
		return nil, err
	}
	signList = append(signList, signByte1)

	signByte2, err := sdkcommon.TxBuilderSignatureJsonDecoder(client.TxConfig, sig2)
	if err != nil {
		return nil, err
	}

	signList = append(signList, signByte2)

	newTx := sdkcommon.NewTxMulSign(client,
		nil,
		msgRequest.GasLimit,
		msgRequest.GasPrice,
		0,
		2,
	)

	txBuilderMultiSign, err := newTx.BuildUnsignedTx(msgRequest.Msg)
	if err != nil {
		return nil, err
	}

	err = newTx.CreateTxMulSign(txBuilderMultiSign, multiSigPubkey, 60, signList)
	if err != nil {
		return nil, err
	}

	txJson, err := sdkcommon.TxBuilderJsonEncoder(client.TxConfig, txBuilderMultiSign)
	if err != nil {
		return nil, err
	}

	txByte, err := sdkcommon.TxBuilderJsonDecoder(client.TxConfig, txJson)
	if err != nil {
		return nil, err
	}

	return client.BroadcastTxCommit(txByte)
}

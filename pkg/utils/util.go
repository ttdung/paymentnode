package utils

import (
	"fmt"
	channelTypes "github.com/AstraProtocol/channel/x/channel/types"
	"github.com/cosmos/cosmos-sdk/client"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/astra-go-sdk/channel"
	sdkcommon "github.com/dungtt-astra/astra-go-sdk/common"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/pkg/errors"
	"log"
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

	openChannelRequest := BuildCommitmentMsg(com, chann, 200000, "0stake")

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

	openChannelRequest := BuildOpenChannelMsg(chann, 200000, "0stake")
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
		log.Println("TxBuilderSignatureJsonDecoder sig1 err:", err.Error())
		return nil, err
	}
	signList = append(signList, signByte1)

	signByte2, err := sdkcommon.TxBuilderSignatureJsonDecoder(client.TxConfig, sig2)
	if err != nil {
		log.Println("TxBuilderSignatureJsonDecoder sig2 err:", err.Error())
		return nil, err
	}

	signList = append(signList, signByte2)

	from := types.AccAddress(multiSigPubkey.Address())
	accNum, accSeq, err := client.AccountRetriever.GetAccountNumberSequence(client, from)
	if err != nil {
		return nil, errors.Wrap(err, "GetAccountNumberSequence")
	}

	newTx := sdkcommon.NewTxMulSign(client,
		nil,
		msgRequest.GasLimit,
		msgRequest.GasPrice,
		accSeq,
		accNum,
	)

	txBuilderMultiSign, err := newTx.BuildUnsignedTx(msgRequest.Msg)
	if err != nil {
		log.Println("BuildUnsignedTx err:", err.Error())
		return nil, err
	}

	err = newTx.CreateTxMulSign(txBuilderMultiSign, multiSigPubkey, common.COINTYPE, signList)
	if err != nil {
		log.Println("CreateTxMulSign err:", err.Error())
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

	log.Println("BroadcastTxCommit str_sig:", string(txByte))
	return client.BroadcastTxCommit(txByte)
}

func BuildAndBroadCastCommiment(client client.Context, chann *common.Channel_st, comm *common.Commitment_st) (*sdk.TxResponse, error) {

	var msgRequest channel.SignMsgRequest

	msgRequest = BuildCommitmentMsg(comm, chann, 200000, "0stake")
	multiSigPubkey := chann.Multisig_Pubkey
	sig1 := comm.StrSigA
	sig2 := comm.StrSigB

	return BuildAndBroadCastMultisigMsg(client, multiSigPubkey, sig1, sig2, msgRequest)
}

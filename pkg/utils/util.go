package utils

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/evmos/ethermint/encoding"
	"github.com/pkg/errors"
	"github.com/ttdung/astra-go-sdk/account"
	"github.com/ttdung/astra-go-sdk/channel"
	sdkcommon "github.com/ttdung/astra-go-sdk/common"
	"github.com/ttdung/channel_v0.46/app"
	channelTypes "github.com/ttdung/channel_v0.46/x/channel/types"
	"github.com/ttdung/paymentnode/config"
	"github.com/ttdung/paymentnode/pkg/common"
	"log"
)

func NewRpcClient(cfg *config.Config) *client.Context {

	sdkConfig := sdk.GetConfig()
	sdkConfig.SetPurpose(44)
	sdkConfig.SetCoinType(cfg.CoinType) // Todo

	bech32PrefixAccAddr := fmt.Sprintf("%v", cfg.PrefixAddress)
	bech32PrefixAccPub := fmt.Sprintf("%vpub", cfg.PrefixAddress)
	bech32PrefixValAddr := fmt.Sprintf("%vvaloper", cfg.PrefixAddress)
	bech32PrefixValPub := fmt.Sprintf("%vvaloperpub", cfg.PrefixAddress)
	bech32PrefixConsAddr := fmt.Sprintf("%vvalcons", cfg.PrefixAddress)
	bech32PrefixConsPub := fmt.Sprintf("%vvalconspub", cfg.PrefixAddress)

	sdkConfig.SetBech32PrefixForAccount(bech32PrefixAccAddr, bech32PrefixAccPub)
	sdkConfig.SetBech32PrefixForValidator(bech32PrefixValAddr, bech32PrefixValPub)
	sdkConfig.SetBech32PrefixForConsensusNode(bech32PrefixConsAddr, bech32PrefixConsPub)

	rpcClient := client.Context{}
	encodingConfig := encoding.MakeConfig(app.ModuleBasics)

	rpcHttp, err := client.NewClientFromNode(cfg.Endpoint)
	if err != nil {
		panic(err)
	}

	rpcClient = rpcClient.
		WithClient(rpcHttp).
		//WithNodeURI(cfg.Endpoint).
		//WithCodec(encodingConfig.Marshaler).
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithChainID(cfg.ChainId).
		WithAccountRetriever(authTypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastSync).
		WithTxConfig(encodingConfig.TxConfig)

	return &rpcClient
}

func BuildCommitmentMsgPartA(com *common.Commitment_st, chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {

	msg := channelTypes.MsgCommitment{
		Creator:      chann.Multisig_Addr,
		MultisigAddr: chann.Multisig_Addr,
		Cointocreator: []*sdk.Coin{{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceA)),
		}},
		Partneraddr: chann.PartB, //peer node
		Numblock:    com.Timelock,
		Creatoraddr: chann.PartA,
		Hashcode:    com.HashcodeB,
		Cointohtlc: []*sdk.Coin{{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceB)),
		}},
		Channelid: chann.Index,
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildCommitmentMsgPartB(com *common.Commitment_st, chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {

	msg := channelTypes.MsgCommitment{
		Creator:      chann.Multisig_Addr,
		MultisigAddr: chann.Multisig_Addr,
		Cointocreator: []*sdk.Coin{{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceB)),
		}},
		Partneraddr: chann.PartA, //peer node
		Numblock:    com.Timelock,
		Creatoraddr: chann.PartB,
		Hashcode:    com.HashcodeA,
		Cointohtlc: []*sdk.Coin{{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceA)),
		}},
		Channelid: chann.Index,
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildAndSignCommitmentMsgPartA(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildCommitmentMsgPartA(com, chann, 200000, "0stake")

	strSig, err := channel.NewChannel(*rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}

	log.Println("Commitment A: ", openChannelRequest)
	log.Println("Sig of commitment: ", strSig)

	return openChannelRequest, strSig, nil
}

func BuildAndSignCommitmentMsgPartB(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildCommitmentMsgPartB(com, chann, 200000, "0stake")

	strSig, err := channel.NewChannel(*rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}

	log.Println("Commitment B: ", openChannelRequest)
	log.Println("Sig of commitment: ", strSig)

	return openChannelRequest, strSig, nil
}

func BuildOpenChannelMsg(chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {
	msg := channelTypes.MsgOpenchannel{
		Creator: chann.Multisig_Addr,
		PartA:   chann.PartA,
		PartB:   chann.PartB,
		CoinA: []*sdk.Coin{{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(chann.Amount_partA)),
		}},
		CoinB: []*sdk.Coin{{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(chann.Amount_partB)),
		}},
		MultisigAddr: chann.Multisig_Addr,
	}

	openChannelRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return openChannelRequest
}

func BuildAndSignOpenChannelMsg(rpcClient *client.Context, account *account.PrivateKeySerialized, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildOpenChannelMsg(chann, 200000, "0stake")
	fmt.Println("openChannelRequest:", openChannelRequest)

	strSig, err := channel.NewChannel(*rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}
	//fmt.Println("openChannelRequest sig:", strSig)

	return openChannelRequest, strSig, nil
}

func BuildMultisigMsgReadyForBroadcast(client *client.Context, multiSigPubkey cryptoTypes.PubKey, sig1, sig2 string, msgRequest channel.SignMsgRequest) ([]byte, error) {

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

	if err != nil {
		return nil, errors.Wrap(err, "GetAccountNumberSequence")
	}

	newTx := sdkcommon.NewTxMulSign(*client,
		nil,
		msgRequest.GasLimit,
		msgRequest.GasPrice,
		0,
		0,
	)

	//log.Println("Message to be Broadcast:", msgRequest)
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
	//log.Println("BroadcastTxCommit txJson:", txJson)

	return sdkcommon.TxBuilderJsonDecoder(client.TxConfig, txJson)
}

func BuildAndBroadCastMultisigMsg(client *client.Context, multiSigPubkey cryptoTypes.PubKey, sig1, sig2 string, msgRequest channel.SignMsgRequest) (*sdk.TxResponse, error) {

	txByte, err := BuildMultisigMsgReadyForBroadcast(client, multiSigPubkey, sig1, sig2, msgRequest)

	if err != nil {
		return nil, err
	}

	return client.BroadcastTxCommit(txByte)
}

func PartABuildFullCommiment(client *client.Context, acc *account.PrivateKeySerialized, chann *common.Channel_st, comm *common.Commitment_st) ([]byte, error) {

	var msgRequest channel.SignMsgRequest

	//msgRequest = BuildCommitmentMsgPartB(comm, chann, 200000, "0stake")
	msgRequest, sig1, err := BuildAndSignCommitmentMsgPartB(client, acc, comm, chann)
	if err != nil {
		return nil, err
	}

	multiSigPubkey := chann.Multisig_Pubkey
	//sig1 := comm.StrSigA
	sig2 := comm.StrSigB
	log.Println("Peer Commitment to be signed:: ", msgRequest.Msg)
	log.Printf("Sig1 %v sig2 %v", sig1, sig2)
	return BuildMultisigMsgReadyForBroadcast(client, multiSigPubkey, sig1, sig2, msgRequest)
}

func PartBBuildFullCommiment(client *client.Context, acc *account.PrivateKeySerialized, chann *common.Channel_st, comm *common.Commitment_st) ([]byte, error) {

	var msgRequest channel.SignMsgRequest

	//msgRequest = BuildCommitmentMsgPartB(comm, chann, 200000, "0stake")
	msgRequest, sig2, err := BuildAndSignCommitmentMsgPartA(client, acc, comm, chann)
	if err != nil {
		return nil, err
	}

	multiSigPubkey := chann.Multisig_Pubkey
	sig1 := comm.StrSigA
	log.Println("Peer Commitment to be signed: ", msgRequest.Msg)
	log.Printf("Sig1 %v sig2 %v", sig1, sig2)
	return BuildMultisigMsgReadyForBroadcast(client, multiSigPubkey, sig1, sig2, msgRequest)
}

func BroadCastCommiment(client client.Context, comm *common.Commitment_st) (*sdk.TxResponse, error) {

	return client.BroadcastTxCommit(comm.TxByteForBroadcast)
}

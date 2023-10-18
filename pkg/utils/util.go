package utils

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	cryptoTypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingTypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/astra-go-sdk/bank"
	"github.com/dungtt-astra/astra-go-sdk/channel"
	sdkcommon "github.com/dungtt-astra/astra-go-sdk/common"
	"github.com/dungtt-astra/astra/v3/app"
	channelTypes "github.com/dungtt-astra/astra/v3/x/channel/types"
	"github.com/dungtt-astra/paymentnode/config"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/evmos/evmos/v12/encoding"
	"github.com/pkg/errors"
	"log"
	"math/big"
	"strings"
	"time"
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

func BuildWithdrawTimeLockPartA(com *common.Commitment_st, chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {
	msg := channelTypes.MsgWithdrawTimelock{
		Creator: chann.PartA,
		To:      chann.PartA,
		Index:   fmt.Sprintf("%v:%v", chann.Multisig_Addr, com.HashcodeA),
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildAndBroadcastWithdrawTimeLockPartA(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st) (*sdk.TxResponse, string, error) {

	withdrawTimelockRequest := BuildWithdrawTimeLockPartA(com, chann, 200000, common.GASPRICE)

	return BroadcastTx(rpcClient, account, withdrawTimelockRequest)

}

func BuildWithdrawHashLockPartA(com *common.Commitment_st, chann *common.Channel_st, secret string, gaslimit uint64, gasprice string) channel.SignMsgRequest {
	msg := channelTypes.MsgWithdrawHashlock{
		Creator: chann.PartA,
		To:      chann.PartA,
		Index:   fmt.Sprintf("%v:%v", chann.Multisig_Addr, com.HashcodeB),
		Secret:  secret,
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildWithdrawHashLockPartB(com *common.Commitment_st, chann *common.Channel_st, secret string, gaslimit uint64, gasprice string) channel.SignMsgRequest {
	msg := channelTypes.MsgWithdrawHashlock{
		Creator: chann.PartB,
		To:      chann.PartB,
		Index:   fmt.Sprintf("%v:%v", chann.Multisig_Addr, com.HashcodeA),
		Secret:  secret,
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildAndBroadcastWithdrawHashLockPartA(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st, secret string) (*sdk.TxResponse, string, error) {

	withdrawTimelockRequest := BuildWithdrawHashLockPartA(com, chann, secret, 200000, common.GASPRICE)

	return BroadcastTx(rpcClient, account, withdrawTimelockRequest)

}

func BuildAndBroadcastWithdrawHashLockPartB(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st, secret string) (*sdk.TxResponse, string, error) {

	withdrawTimelockRequest := BuildWithdrawHashLockPartB(com, chann, secret, 200000, common.GASPRICE)

	return BroadcastTx(rpcClient, account, withdrawTimelockRequest)

}

func TransferTokenWithPrivateKey(privateKey string,
	rpcClient client.Context,
	tokenSymbol string,
	coinType uint32,
	receiver string,
	amount *big.Int,
	gaslimit uint64,
	gasprice string) (*sdk.TxResponse, error) {

	bankClient := bank.NewBank(rpcClient, tokenSymbol, coinType)

	request := &bank.TransferRequest{
		PrivateKey: privateKey,
		Receiver:   receiver,
		Amount:     amount,
		GasLimit:   gaslimit,
		GasPrice:   gasprice,
	}

	txBuilder, err := bankClient.TransferRawDataWithPrivateKey(request)
	if err != nil {
		panic(err)
	}

	txJson, err := sdkcommon.TxBuilderJsonEncoder(rpcClient.TxConfig, txBuilder)
	if err != nil {
		panic(err)
	}

	txByte, err := sdkcommon.TxBuilderJsonDecoder(rpcClient.TxConfig, txJson)
	if err != nil {
		panic(err)
	}

	//txHash := sdkcommon.TxHash(txByte)

	return rpcClient.BroadcastTxCommit(txByte)

}

func BroadcastTx(client *client.Context, account *account.PrivateKeySerialized, request channel.SignMsgRequest) (*sdk.TxResponse, string, error) {

	newTx := sdkcommon.NewTx(
		*client,
		account,
		request.GasLimit,
		request.GasPrice,
	)

	txBuilder, err := newTx.BuildUnsignedTx(request.Msg)
	if err != nil {
		return nil, "", err
	}

	err = newTx.SignTx(txBuilder)
	if err != nil {
		return nil, "", err
	}

	txJson, err := sdkcommon.TxBuilderJsonEncoder(client.TxConfig, txBuilder)
	if err != nil {
		panic(err)
	}

	fmt.Println("Tx rawData", string(txJson))

	txByte, err := sdkcommon.TxBuilderJsonDecoder(client.TxConfig, txJson)
	if err != nil {
		panic(err)
	}

	txHash := sdkcommon.TxHash(txByte)
	fmt.Println("txHash", txHash)

	//fmt.Println(ethCommon.BytesToHash(txByte).String())

	res, err := client.BroadcastTxCommit(txByte)
	if err != nil {
		panic(err)
	}

	fmt.Println(res)

	return res, txHash, err
}

func BuildCommitmentMsgPartA(com *common.Commitment_st, chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {

	var coinA, coinB []*sdk.Coin
	coinA = make([]*sdk.Coin, len(chann.Denom))
	coinB = make([]*sdk.Coin, len(chann.Denom))

	for i := 0; i < len(chann.Denom); i++ {
		coinA[i] = &sdk.Coin{
			Denom:  chann.Denom[i],
			Amount: sdk.NewInt(int64(com.BalanceA[i])),
		}
		coinB[i] = &sdk.Coin{
			Denom:  chann.Denom[i],
			Amount: sdk.NewInt(int64(com.BalanceB[i])),
		}
	}

	msg := channelTypes.MsgCommitment{
		Creator:       chann.Multisig_Addr,
		MultisigAddr:  chann.Multisig_Addr,
		Cointocreator: coinA,
		Partneraddr:   chann.PartB, //peer node
		Numblock:      com.Timelock,
		Creatoraddr:   chann.PartA,
		Hashcode:      com.HashcodeB,
		Cointohtlc:    coinB,
		Channelid:     chann.ChannelID,
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildCommitmentMsgPartB(com *common.Commitment_st, chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {

	var cointocreator, cointohtlc []*sdk.Coin
	cointocreator = make([]*sdk.Coin, len(chann.Denom))
	cointohtlc = make([]*sdk.Coin, len(chann.Denom))

	for i := 0; i < len(chann.Denom); i++ {
		cointocreator[i] = &sdk.Coin{
			Denom:  chann.Denom[i],
			Amount: sdk.NewInt(int64(com.BalanceB[i])),
		}
		cointohtlc[i] = &sdk.Coin{
			Denom:  chann.Denom[i],
			Amount: sdk.NewInt(int64(com.BalanceA[i])),
		}
	}

	msg := channelTypes.MsgCommitment{
		Creator:       chann.Multisig_Addr,
		MultisigAddr:  chann.Multisig_Addr,
		Cointocreator: cointocreator,
		Partneraddr:   chann.PartA, //peer node
		Numblock:      com.Timelock,
		Creatoraddr:   chann.PartB,
		Hashcode:      com.HashcodeA,
		Cointohtlc:    cointohtlc,
		Channelid:     chann.ChannelID,
	}

	commitmentRequest := channel.SignMsgRequest{
		Msg:      &msg,
		GasLimit: gaslimit,
		GasPrice: gasprice,
	}

	return commitmentRequest
}

func BuildAndSignCommitmentMsgPartA(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildCommitmentMsgPartA(com, chann, 200000, common.GASPRICE)

	log.Println("BuildAndSignCommitmentMsgPartA : ")
	//strSig, err := channel.NewChannel(*rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	strSig, err := channel.NewChannel(*rpcClient).SignCommitmentMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}

	//log.Println("Commitment A: ", openChannelRequest)
	//log.Println("Sig of commitment: ", strSig)

	return openChannelRequest, strSig, nil
}

func BuildAndSignCommitmentMsgPartB(rpcClient *client.Context, account *account.PrivateKeySerialized, com *common.Commitment_st, chann *common.Channel_st) (channel.SignMsgRequest, string, error) {

	openChannelRequest := BuildCommitmentMsgPartB(com, chann, 200000, common.GASPRICE)

	log.Println("BuildAndSignCommitmentMsgPartB : ")
	log.Println("BuildAndSignCommitmentMsgPartB : openChannelRequest", openChannelRequest)
	//strSig, err := channel.NewChannel(*rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)

	for true {
		strSig, err := channel.NewChannel(*rpcClient).SignCommitmentMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				time.Sleep(time.Second)
				log.Println("BuildAndSignCommitmentMsgPartB err: ", err.Error())
				continue
			}
			fmt.Printf("BuildAndSignCommitmentMsgPartB error: %v\n", err.Error())
			return openChannelRequest, "", err
		} else {
			return openChannelRequest, strSig, nil
		}
	}
	//log.Println("Commitment B: ", openChannelRequest)
	//log.Println("Sig of commitment: ", strSig)

	return openChannelRequest, "", errors.New("unknown")
}

func BuildOpenChannelMsg(chann *common.Channel_st, gaslimit uint64, gasprice string) channel.SignMsgRequest {

	var coinA, coinB []*sdk.Coin
	coinA = make([]*sdk.Coin, len(chann.Denom))
	coinB = make([]*sdk.Coin, len(chann.Denom))

	for i := 0; i < len(chann.Denom); i++ {
		coinA[i] = &sdk.Coin{
			Denom:  chann.Denom[i],
			Amount: sdk.NewInt(int64(chann.Amount_partA[i])),
		}
		coinB[i] = &sdk.Coin{
			Denom:  chann.Denom[i],
			Amount: sdk.NewInt(int64(chann.Amount_partB[i])),
		}
	}

	msg := channelTypes.MsgOpenchannel{
		Creator:      chann.Multisig_Addr,
		PartA:        chann.PubkeyA.String(), //chann.PartA,
		PartB:        chann.PubkeyB.String(), //chann.PartB,
		CoinA:        coinA,
		CoinB:        coinB,
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

	//chann1 := *chann
	//p, _ := pubkey.NewPKAccount("EthPubKeySecp256k1{02EF87B47FCF61090764C513E611FBBC30C202F0C85A84F05C84F7318632542B63}")
	//chann1.PubkeyA = p.PublicKey()
	//openChannelRequest := BuildOpenChannelMsg(&chann1, 200000, common.GASPRICE)
	openChannelRequest := BuildOpenChannelMsg(chann, 200000, common.GASPRICE)

	fmt.Println("BuildAndSignOpenChannelMsg:openChannelRequestMsg:", openChannelRequest)
	fmt.Println("BuildAndSignOpenChannelMsg:account_Addr:", account.AccAddress().String())
	fmt.Println("BuildAndSignOpenChannelMsg:chann.Multisig_Pubkey:", chann.Multisig_Pubkey.Address().String())

	strSig, err := channel.NewChannel(*rpcClient).SignMultisigMsg(openChannelRequest, account, chann.Multisig_Pubkey)
	if err != nil {
		fmt.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return openChannelRequest, "", err
	}
	//fmt.Println("openChannelRequest sig:", strSig)

	return openChannelRequest, strSig, nil
}

func BuildCommitmentMsgReadyForBroadcast(client *client.Context, multiSigPubkey cryptoTypes.PubKey, sig1, sig2 string, msgRequest channel.SignMsgRequest) ([]byte, error) {

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

	accSeq := uint64(0)
	accNum := uint64(0)
	accNum, accSeq, err = client.AccountRetriever.GetAccountNumberSequence(
		*client,
		sdk.AccAddress(multiSigPubkey.Address()))
	if err != nil {
		return []byte(""), errors.Wrap(err, "GetAccountNumberSequence")
	}

	log.Printf("BuildCommitmentMsgReadyForBroadcast: accNum %v accSeq+1 %v", accNum, accSeq+1)
	newTx := sdkcommon.NewTxMulSign(*client,
		nil,
		msgRequest.GasLimit,
		msgRequest.GasPrice,
		accSeq+1,
		accNum,
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

	accSeq := uint64(0)
	accNum := uint64(0)

	log.Printf("BuildMultisigMsgReadyForBroadcast: accNum %v accSeq+1 %v", accNum, accSeq+1)
	newTx := sdkcommon.NewTxMulSign(*client,
		nil,
		msgRequest.GasLimit,
		msgRequest.GasPrice,
		accSeq,
		accNum,
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

	//msgRequest = BuildCommitmentMsgPartB(comm, chann, 200000, common.GASPRICE)
	msgRequest, sig1, err := BuildAndSignCommitmentMsgPartB(client, acc, comm, chann)
	if err != nil {
		return nil, err
	}

	multiSigPubkey := chann.Multisig_Pubkey
	//sig1 := comm.StrSigA
	sig2 := comm.StrSigB
	//log.Println("Peer Commitment to be signed:: ", msgRequest.Msg)
	//log.Printf("Sig1 %v sig2 %v", sig1, sig2)
	return BuildCommitmentMsgReadyForBroadcast(client, multiSigPubkey, sig1, sig2, msgRequest)
}

func PartBBuildFullCommiment(client *client.Context, acc *account.PrivateKeySerialized, chann *common.Channel_st, comm *common.Commitment_st) ([]byte, error) {

	var msgRequest channel.SignMsgRequest

	//msgRequest = BuildCommitmentMsgPartB(comm, chann, 200000, common.GASPRICE)
	msgRequest, sig2, err := BuildAndSignCommitmentMsgPartA(client, acc, comm, chann)
	if err != nil {
		return nil, err
	}

	multiSigPubkey := chann.Multisig_Pubkey
	sig1 := comm.StrSigA
	//log.Println("Peer Commitment to be signed: ", msgRequest.Msg)
	//log.Printf("Sig1 %v sig2 %v", sig1, sig2)
	return BuildCommitmentMsgReadyForBroadcast(client, multiSigPubkey, sig1, sig2, msgRequest)
}

func BroadCastCommiment(client client.Context, comm *common.Commitment_st) (*sdk.TxResponse, error) {

	return client.BroadcastTxCommit(comm.TxByteForBroadcast)
}

func GetBalance(rpcClient *client.Context, addr string) (*big.Int, error) {

	bankClient := bank.NewBank(*rpcClient, common.DENOM, common.COINTYPE)

	return bankClient.Balance(addr)
}

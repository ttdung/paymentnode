package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	channelTypes "github.com/AstraProtocol/channel/x/channel/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/astra-go-sdk/channel"
	"github.com/dungtt-astra/paymentnode/config"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/dungtt-astra/paymentnode/pkg/utils"
	node "github.com/dungtt-astra/paymentnode/proto"
	"github.com/evmos/ethermint/app"
	"github.com/evmos/ethermint/encoding"
	ethermintTypes "github.com/evmos/ethermint/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"io"
	"log"
	"time"
)

var (
	serverAddr = flag.String("server_addr", "localhost:50005", "The server address in the format of host:port")
)

//	type MachineClient struct {
//		//machine.UnimplementedMachineClient
//		stream    machine.Machine_ExecuteClient
//		account   *account.PrivateKeySerialized
//		denom     string
//		amount    int64
//		version   string
//		channel   data.Msg_Channel
//		cn        *channel.Channel
//		rpcClient client.Context
//	}
type Balance struct {
	partA float64
	partB float64
}

var balance_map = make(map[string]*Balance)

var mmemonic = "baby cancel magnet patient urge regular ribbon scorpion buyer zoo muffin style echo flock soda text door multiply present vocal budget employ target radar"

var waitc = make(chan struct{})

var comm_map = make(map[string]*common.Commitment_st)

var pre_commid string
var channel_st common.Channel_st

type MachineClient struct {
	//machine.UnimplementedMachineClient
	stream  node.Node_OpenStreamClient
	account *account.PrivateKeySerialized
	denom   string
	amount  int64
	version string
	//channel   data.Msg_Channel
	rpcClient client.Context
	client    node.NodeClient
	passcode  string
	timelock  uint64
}

var cfg = &config.Config{
	ChainId:       "astra_11110-1",
	Endpoint:      "http://128.199.238.171:26657",
	CoinType:      60,
	PrefixAddress: "astra",
	TokenSymbol:   "aastra",
}

func (c *MachineClient) Init(stream node.Node_OpenStreamClient) {

	c.passcode = "secret string"
	c.stream = stream

	c.denom = "aastra"
	c.amount = 0
	c.version = "0.1"
	c.timelock = 100

	// channel
	channel_st.PartB = "astra1xu9rev8fw0y9wrutv0llthwjsmppp9nn0su4uh"

	// set astra address
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetPurpose(44)
	sdkConfig.SetCoinType(ethermintTypes.Bip44CoinType)

	bech32PrefixAccAddr := fmt.Sprintf("%v", cfg.PrefixAddress)
	bech32PrefixAccPub := fmt.Sprintf("%vpub", cfg.PrefixAddress)
	bech32PrefixValAddr := fmt.Sprintf("%vvaloper", cfg.PrefixAddress)
	bech32PrefixValPub := fmt.Sprintf("%vvaloperpub", cfg.PrefixAddress)
	bech32PrefixConsAddr := fmt.Sprintf("%vvalcons", cfg.PrefixAddress)
	bech32PrefixConsPub := fmt.Sprintf("%vvalconspub", cfg.PrefixAddress)

	sdkConfig.SetBech32PrefixForAccount(bech32PrefixAccAddr, bech32PrefixAccPub)
	sdkConfig.SetBech32PrefixForValidator(bech32PrefixValAddr, bech32PrefixValPub)
	sdkConfig.SetBech32PrefixForConsensusNode(bech32PrefixConsAddr, bech32PrefixConsPub)

	acc, err := account.NewAccount(60).ImportAccount(mmemonic)
	if err != nil {
		log.Println("ImportAccount Err:", err.Error())
		return
	}

	c.account = acc

	//
	rpcClient := client.Context{}
	encodingConfig := encoding.MakeConfig(app.ModuleBasics)

	rpcHttp, err := client.NewClientFromNode(cfg.Endpoint)
	if err != nil {
		panic(err)
	}

	c.rpcClient = rpcClient.
		WithClient(rpcHttp).
		//WithNodeURI(c.endpoint).
		WithCodec(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithChainID(cfg.ChainId).
		WithAccountRetriever(authTypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastSync).
		WithTxConfig(encodingConfig.TxConfig)

}

func connect(serverAddr *string) (node.NodeClient, node.Node_OpenStreamClient, *grpc.ClientConn, error) {
	flag.Parse()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	client := node.NewNodeClient(conn)
	//ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	ctx := context.Background()
	//defer cancel()

	stream, err := client.OpenStream(ctx)
	if err != nil {
		log.Fatalf("%v.Execute(ctx) = %v, %v: ", client, stream, err)
	}

	return client, stream, conn, err
}

type NotifReqPaymentSt struct {
	ChannelID string
	SendAmt   uint64
	RecvAmt   uint64
	Nonce     uint64
	Hashcode  string
}

func eventHandler(c *MachineClient) {

	channelID := fmt.Sprintf("%v:%v:%v", c.account.AccAddress().String(), channel_st.PartB, c.denom)

	msg := &node.Msg{
		Type: node.MsgType_REG_CHANNEL,
		Data: []byte(channelID),
	}
	c.stream.Send(msg)

	for {
		msg, err := c.stream.Recv()
		if err == io.EOF {
			log.Println("EOF")
			close(waitc)
			return
		}
		if err != nil {
			log.Printf("Err: %v", err)
			close(waitc)
			return
		}
		log.Printf("output: %v", msg.String())

		msgtype := msg.GetType()
		data := msg.GetData()
		//cmd_type := util.CmdType(cmd)
		//
		switch msgtype {
		case node.MsgType_REQ_PAYMENT:
			var rp NotifReqPaymentSt
			json.Unmarshal(data, &rp)
			log.Println("Notif Req payment....")
			log.Println("ChannelID:", rp.ChannelID)
			log.Println("SendAmt:", rp.SendAmt)
			c.makePayment(rp)
		//
		//case util.MSG_ERROR:
		//	log.Fatalf("Error: %v", data.Fields[field.Error].GetStringValue())

		default:
			close(waitc)
			//return status.Errorf(codes.Unimplemented, "Operation '%s' not implemented yet", operator)
			log.Println(codes.Unimplemented, "Operation '%s' not implemented yet ", msgtype)

			return
		}
	}
}

func (c *MachineClient) handleResponseOpenChannel(in *node.MsgResOpenChannel) (string, error) {

	return "", nil
}

func (c *MachineClient) GenerateHashcode(commitID string) (string, string) {

	now := time.Now()
	secret := fmt.Sprintf("%v:%v:%v", c.passcode, now, commitID)

	hash := sha256.Sum256([]byte(secret))
	hashcode := base64.StdEncoding.EncodeToString(hash[:])

	return secret, hashcode
}

func (c *MachineClient) buildChannelInfo(req *node.MsgReqOpenChannel, res *node.MsgResOpenChannel) (common.Channel_st, error) {
	log.Println("server pubkey:", res.Pubkey)
	peerPubkey, err := account.NewPKAccount(res.Pubkey)

	multisigAddr, multiSigPubkey, err := account.NewAccount(60).CreateMulSignAccountFromTwoAccount(c.account.PublicKey(), peerPubkey.PublicKey(), 2)
	if err != nil {
		return common.Channel_st{}, err
	}

	channelID := fmt.Sprintf("%v:%v:%v", req.PartA_Addr, req.PartB_Addr, req.Denom)

	chann := common.Channel_st{
		Index:           channelID,
		Multisig_Addr:   multisigAddr,
		Multisig_Pubkey: multiSigPubkey,
		PartA:           req.PartA_Addr,
		PartB:           req.PartB_Addr,
		PubkeyA:         c.account.PublicKey(),
		PubkeyB:         peerPubkey.PublicKey(),
		Denom:           req.Denom,
		Amount_partA:    float64(req.Deposit_Amt),
		Amount_partB:    float64(res.Deposit_Amt),
	}

	return chann, nil
}

func (c *MachineClient) buildCommitmentInfo(req *node.MsgReqOpenChannel, res *node.MsgResOpenChannel, secret string) *common.Commitment_st {
	com := &common.Commitment_st{
		ChannelID:   fmt.Sprintf("%v:%v:%v", req.PartA_Addr, req.PartB_Addr, req.Denom),
		Denom:       req.Denom,
		BalanceA:    float64(req.Deposit_Amt + req.FirstRecv - req.FirstSend),
		BalanceB:    float64(res.Deposit_Amt - req.FirstRecv),
		HashcodeA:   req.Hashcode,
		HashcodeB:   res.Hashcode,
		SecretA:     secret,
		SecretB:     "",
		PenaltyA_Tx: "",
		PenaltyB_Tx: "",
		Timelock:    c.timelock,
		Nonce:       res.Nonce,
	}

	return com
}

func (c *MachineClient) buildAndSignCommitmentMsg(com *common.Commitment_st, chann *common.Channel_st) (string, error) {

	msg := channelTypes.MsgCommitment{
		Creator: chann.Multisig_Addr,
		From:    chann.PartA,
		CoinToCreator: &sdk.Coin{
			Denom:  chann.Denom,
			Amount: sdk.NewInt(int64(com.BalanceA)),
		},
		ToTimelockAddr: chann.PartB, //peer node
		Timelock:       100,
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
		GasLimit: 200000,
		GasPrice: "25aastra",
	}
	//fmt.Println("openChannelRequest:", openChannelRequest)

	strSig, err := channel.NewChannel(c.rpcClient).SignMultisigMsg(openChannelRequest, c.account, chann.Multisig_Pubkey)
	if err != nil {
		log.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return "", err
	}

	return strSig, nil
}

func (c *MachineClient) buildAndSignOpenChannelMsg(com *common.Commitment_st, chann *common.Channel_st) (string, error) {
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
		GasLimit: 200000,
		GasPrice: "25aastra",
	}
	//fmt.Println("openChannelRequest:", openChannelRequest)

	strSig, err := channel.NewChannel(c.rpcClient).SignMultisigMsg(openChannelRequest, c.account, chann.Multisig_Pubkey)
	if err != nil {
		log.Printf("SignMultisigTxFromOneAccount error: %v\n", err)
		return "", err
	}

	return strSig, nil
}

func (c *MachineClient) buildConfirmMsg(com *common.Commitment_st, channinfo *common.Channel_st) *node.MsgConfirmOpenChannel {

	msg := &node.MsgConfirmOpenChannel{
		Type: node.MsgType_CONFIRM_OPENCHANNEL,
	}

	// build commitment msg
	_, com_sig, err := utils.BuildAndSignCommitmentMsg(c.rpcClient, c.account, com, channinfo)
	if err != nil {
		msg.Type = node.MsgType_ERROR
		msg.CommitmentSig = err.Error()
		return msg
	}

	// bụild openchannel tx msg
	msgopen, openchannel_sig, err := utils.BuildAndSignOpenChannelMsg(c.rpcClient, c.account, channinfo)
	if err != nil {
		msg.Type = node.MsgType_ERROR
		msg.CommitmentSig = err.Error()
		return msg
	}
	log.Println("MsgOpen: ", msgopen)
	log.Println("Sig MsgOpen: ", openchannel_sig)

	msg.ChannelID = channinfo.Index
	msg.CommitmentSig = com_sig
	msg.OpenChannelTxSig = openchannel_sig

	return msg
}

func (c *MachineClient) makePayment(rp NotifReqPaymentSt) error {

	//balance_map
	secret, hashcode := c.GenerateHashcode(rp.ChannelID)

	com := &common.Commitment_st{
		ChannelID:   rp.ChannelID,
		Denom:       c.denom,
		BalanceA:    balance_map[rp.ChannelID].partA + float64(rp.SendAmt-rp.RecvAmt),
		BalanceB:    balance_map[rp.ChannelID].partB + float64(rp.RecvAmt-rp.SendAmt),
		HashcodeA:   hashcode,
		HashcodeB:   rp.Hashcode,
		SecretA:     secret,
		SecretB:     "",
		PenaltyA_Tx: "",
		PenaltyB_Tx: "",
		Timelock:    c.timelock,
		Nonce:       rp.Nonce,
	}

	commid := fmt.Sprintf("%v:%v", rp.ChannelID, rp.Nonce)
	comm_map[commid] = com

	// build commitment msg
	_, com_sig, err := utils.BuildAndSignCommitmentMsg(c.rpcClient, c.account, com, &channel_st)
	if err != nil {
		return err
	}

	msgRP := &node.MsgReqPayment{
		ChannelID:     rp.ChannelID,
		SendAmt:       rp.RecvAmt,
		RecvAmt:       rp.SendAmt,
		Hashcode:      hashcode,
		CommitmentID:  commid,
		CommitmentSig: com_sig,
	}

	resPayment, err := c.client.RequestPayment(context.Background(), msgRP)
	if err != nil {
		return err
	}

	log.Println("Response RequestPayment", resPayment)

	msgCP := &node.MsgConfirmPayment{
		SecretPreComm: comm_map[pre_commid].SecretA,
	}

	pre_commid = commid

	_, err = c.client.ConfirmPayment(context.Background(), msgCP)

	return err
}

func (c *MachineClient) openChannel() error {

	//pubkey := c.account.PublicKey().String()
	channelID := fmt.Sprintf("%v:%v:%v", c.account.AccAddress().String(), channel_st.PartB, c.denom)
	secret, hashcode := c.GenerateHashcode(channelID)

	req := &node.MsgReqOpenChannel{
		Version:      c.version,
		PartA_Addr:   c.account.AccAddress().String(),
		PartB_Addr:   channel_st.PartB,
		PubkeyA:      c.account.PublicKey().String(),
		Deposit_Amt:  0,
		Denom:        c.denom,
		Hashcode:     hashcode,
		PeerNodeAddr: ":50005",
		FirstSend:    0,
		FirstRecv:    1,
	}

	balance_map[channelID] = &Balance{
		float64(req.Deposit_Amt + req.FirstRecv - req.FirstSend),
		0,
	}

	log.Println("RequestOpenChannel ...")
	res, err := c.client.RequestOpenChannel(context.Background(), req)
	if err != nil {
		log.Println(err)
		return nil
	}

	cominfo := c.buildCommitmentInfo(req, res, secret)

	comid := fmt.Sprintf("%v:%v", cominfo.ChannelID, cominfo.Nonce)
	comm_map[comid] = cominfo
	pre_commid = comid
	balance_map[cominfo.ChannelID] = &Balance{
		cominfo.BalanceA,
		cominfo.BalanceB,
	}

	channel_st, err := c.buildChannelInfo(req, res)
	if err != nil {
		log.Println(err)
	}

	// todo build confirm Msg

	log.Println("response RequestOpenChannel info...")
	log.Println(res)

	confirmMsg := c.buildConfirmMsg(cominfo, &channel_st)

	log.Println("ConfirmOpenChannel ...")
	resConfirm, err := c.client.ConfirmOpenChannel(context.Background(), confirmMsg)
	if err != nil {
		log.Println(err)
	}

	log.Println("res ConfirmOpenChannelsto...")
	log.Println("TxHash;", resConfirm.TxHash)
	log.Println("Code;", resConfirm.Code)

	return nil
}

func main() {

	client, stream, conn, err := connect(serverAddr)

	if err != nil {
		log.Printf("Err: %v", err)
		return
	}

	c := new(MachineClient)
	c.client = client

	c.Init(stream)

	go eventHandler(c)

	c.openChannel()

	time.Sleep(500 * time.Millisecond)

	if err := stream.CloseSend(); err != nil {
		log.Fatalf("%v.CloseSend() got error %v, want %v", stream, err, nil)
	}

	<-waitc
	conn.Close()
}

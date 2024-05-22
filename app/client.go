package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/ttdung/astra-go-sdk/account"
	"github.com/ttdung/paymentnode/config"
	"github.com/ttdung/paymentnode/pkg/common"
	"github.com/ttdung/paymentnode/pkg/utils"
	node "github.com/ttdung/paymentnode/proto"
	"google.golang.org/grpc"
	//"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

const SINGLE_HOP = 0
const MULTI_HOP = 1

var nonce = uint64(0)
var (
	serverAddr = flag.String("server_addr", "localhost:50005", "The server address in the format of host:port")
)

type Balance struct {
	partA []uint64
	partB []uint64
}

var balance_map = make(map[string]*Balance)

var mmemonic = []string{
	"plastic teach expect wolf kit misery quarter episode wide begin season flip medal ginger item vague repeat super deputy dad shoe bright dry core",
}

var partB = "cosmos16ydvmd8a85y5xrc2y3pzafz8wm9v6lnewukekn" // a12

var waitc = make(chan struct{})

var comm_map = make(map[string]*common.Commitment_st)

var pre_commid string
var channel_st common.Channel_st

type MachineClient struct {
	//machine.UnimplementedMachineClient
	stream  node.Node_OpenStreamClient
	account *account.PrivateKeySerialized
	denom   []string
	amount  []uint64
	version string
	//channel   data.Msg_Channel
	rpcClient *client.Context
	client    node.NodeClient
	passcode  string
	timelock  uint64
}

//var cfg = &config.Config{
//	ChainId:       "astra_11115-1",
//	Endpoint:      "http://localhost:26657",
//	CoinType:      common.COINTYPE, //ethermintTypes.Bip44CoinType,
//	PrefixAddress: "astra",
//	TokenSymbol:   []string{common.DENOM},
//}

var cfg = &config.Config{
	ChainId:       "channel_v0.46",
	Endpoint:      "http://localhost:26657",
	CoinType:      common.COINTYPE,
	PrefixAddress: "cosmos",
	TokenSymbol:   []string{"stake"},
}

func NewMachineClient(stream node.Node_OpenStreamClient,
	client node.NodeClient,
	account *account.PrivateKeySerialized) *MachineClient {

	return &MachineClient{
		stream,
		account,
		[]string{common.DENOM},
		[]uint64{0},
		"0.1",
		utils.NewRpcClient(cfg),
		client,
		"secret string",
		uint64(common.TIMELOCK),
	}
}

func (c *MachineClient) Init() {

	//fmt.Println("Start my init")
	//
	//c.passcode = "secret string"
	////c.stream = stream
	//
	//c.denom = common.DENOM
	//c.amount = 0
	//c.version = "0.1"
	//c.timelock = uint64(common.TIMELOCK)

	// channel
	channel_st.PartB = partB

	//acc, err := account.NewAccount(common.COINTYPE).ImportAccount(mmemonic)
	//if err != nil {
	//	log.Println("ImportAccount Err:", err.Error())
	//	return
	//}
	//
	//c.account = acc

}

func (c *MachineClient) GetAccount() *account.PrivateKeySerialized {
	return c.account
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
	Type       uint32
	SenderAddr string
	ChannelID  string
	SendAmt    []uint64
	RecvAmt    []uint64
	Nonce      uint64
	Hashcode   string
}

func (c *MachineClient) handleResponseOpenChannel(in *node.MsgResOpenChannel) (string, error) {

	return "", nil
}

func (c *MachineClient) GenerateHashcode(commitID string) (string, string) {

	now := time.Now()

	secret := fmt.Sprintf("%v:%v:%v", c.passcode, now, commitID)

	// TODO remove on production, this is for testing purpose
	secret = "abcd"

	hash := sha256.Sum256([]byte(secret))
	hashcode := base64.StdEncoding.EncodeToString(hash[:])

	return secret, hashcode
}

func (c *MachineClient) buildChannelInfo(req *node.MsgReqOpenChannel, res *node.MsgResOpenChannel) (common.Channel_st, error) {

	peerPubkey, err := account.NewPKAccount(res.Pubkey)

	log.Println("Peer pubkey:", peerPubkey.AccAddress())
	log.Println("Client pubkey:", c.account.PublicKey().Address())
	multisigAddr, multiSigPubkey, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(peerPubkey.PublicKey(), c.account.PublicKey(), 2)
	if err != nil {
		return common.Channel_st{}, err
	}

	log.Println("MultisigAddr:", multisigAddr)
	log.Println("multiSigPubkey:", multiSigPubkey.Address())

	channelID := fmt.Sprintf("%v:%v", multisigAddr, req.Sequence)

	chann := common.Channel_st{
		ChannelID:       channelID,
		Multisig_Addr:   multisigAddr,
		Multisig_Pubkey: multiSigPubkey,
		PartA:           req.PartA_Addr,
		PartB:           req.PartB_Addr,
		PubkeyA:         c.account.PublicKey(),
		PubkeyB:         peerPubkey.PublicKey(),
		Denom:           req.Denom,
		Amount_partA:    req.Deposit_Amt,
		Amount_partB:    res.Deposit_Amt,
	}

	return chann, nil
}

func (c *MachineClient) buildCommitmentInfo(req *node.MsgReqOpenChannel, res *node.MsgResOpenChannel, secret string) *common.Commitment_st {
	peerPubkey, err := account.NewPKAccount(res.Pubkey)

	multisigAddr, _, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(peerPubkey.PublicKey(), c.account.PublicKey(), 2)
	if err != nil {
		return nil
	}

	var balanceA, balanceB []uint64
	balanceA = make([]uint64, len(req.Deposit_Amt))
	balanceB = make([]uint64, len(req.Deposit_Amt))

	for i := 0; i < len(req.Deposit_Amt); i++ {
		balanceA[i] = req.Deposit_Amt[i] + req.FirstRecv[i] - req.FirstSend[i]
		balanceB[i] = req.Deposit_Amt[i] - req.FirstRecv[i] + req.FirstSend[i]
	}

	com := &common.Commitment_st{
		ChannelID:   fmt.Sprintf("%v:%v", multisigAddr, req.Denom),
		Denom:       req.Denom,
		BalanceA:    balanceA,
		BalanceB:    balanceB,
		HashcodeA:   req.Hashcode,
		HashcodeB:   res.Hashcode,
		SecretA:     secret,
		SecretB:     "",
		StrSigA:     "",
		StrSigB:     res.Commitment_Sig,
		PenaltyA_Tx: "",
		PenaltyB_Tx: "",
		Timelock:    c.timelock,
		Nonce:       res.Nonce,
	}

	return com
}

func (c *MachineClient) buildConfirmMsg(com *common.Commitment_st, channinfo *common.Channel_st) *node.MsgConfirmOpenChannel {

	msg := &node.MsgConfirmOpenChannel{
		Type: node.MsgType_CONFIRM_OPENCHANNEL,
	}

	// build commitment msg
	_, com_sig, err := utils.BuildAndSignCommitmentMsgPartA(c.rpcClient, c.account, com, channinfo)
	if err != nil {
		msg.Type = node.MsgType_ERROR
		msg.CommitmentSig = err.Error()
		return msg
	}

	// bụild openchannel tx msg
	_, openchannel_sig, err := utils.BuildAndSignOpenChannelMsg(c.rpcClient, c.account, channinfo)
	if err != nil {
		msg.Type = node.MsgType_ERROR
		msg.CommitmentSig = err.Error()
		return msg
	}

	msg.ChannelID = channinfo.ChannelID
	msg.CommitmentSig = com_sig
	msg.OpenChannelTxSig = openchannel_sig

	return msg
}

func (c *MachineClient) makeReqDirectPayment(rp NotifReqPaymentSt) error {

	//balance_map
	secret, hashcode := c.GenerateHashcode(rp.ChannelID)

	var balanceA, balanceB []uint64
	balanceA = make([]uint64, len(rp.SendAmt))
	balanceB = make([]uint64, len(rp.SendAmt))

	for i := 0; i < len(rp.SendAmt); i++ {
		balanceA[i] = balance_map[rp.ChannelID].partA[i] + rp.SendAmt[i] - rp.RecvAmt[i]
		balanceB[i] = balance_map[rp.ChannelID].partB[i] + rp.RecvAmt[i] - rp.SendAmt[i]
	}

	com := &common.Commitment_st{
		ChannelID:   rp.ChannelID,
		Denom:       c.denom,
		BalanceA:    balanceA,
		BalanceB:    balanceB,
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
	//log.Println("makePayment:", channel_st)
	_, com_sig, err := utils.BuildAndSignCommitmentMsgPartA(c.rpcClient, c.account, com, &channel_st)
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

	_, err = c.client.RequestPayment(context.Background(), msgRP)
	if err != nil {
		log.Println("makePayment:RequestPayment err:", err.Error())
		return err
	}

	//log.Println("Response RequestPayment", resPayment)

	msgCP := &node.MsgConfirmPayment{
		ChannelID:     rp.ChannelID,
		CommID:        commid,
		SecretPreComm: comm_map[pre_commid].SecretA,
	}

	//log.Println("MsgConfirmPayment Msg:", msgCP)

	pre_commid = commid

	_, err = c.client.ConfirmPayment(context.Background(), msgCP)

	return err
}

func (c *MachineClient) makeMultiHopPayment(rp NotifReqPaymentSt) error {

	return nil
}

func (c *MachineClient) makePayment(rp NotifReqPaymentSt) error {

	if rp.Type == SINGLE_HOP {
		return c.makeReqDirectPayment(rp)
	} else {
		return c.makeMultiHopPayment(rp)
	}
}

func (c *MachineClient) buildInfo(req *node.MsgReqOpenChannel, res *node.MsgResOpenChannel, secret string) *node.MsgConfirmOpenChannel {

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

	log.Println("Client build channel info... channel_st:")
	log.Println(channel_st)

	confirmMsg := c.buildConfirmMsg(cominfo, &channel_st)
	comm_map[comid].StrSigA = confirmMsg.CommitmentSig

	log.Println("Client start PartABuildFullCommiment()..")
	txbyte, err := utils.PartABuildFullCommiment(c.rpcClient, c.GetAccount(), &channel_st, comm_map[comid])
	if err != nil {
		panic(err)
	}

	comm_map[comid].TxByteForBroadcast = txbyte

	return confirmMsg
}

func (c *MachineClient) openChannel() error {

	//pubkey := c.account.PublicKey().String()
	//log.Println("PartA addr:", c.account.AccAddress().String()) //client
	//log.Println("PartB addr:", channel_st.PartB) // node

	randomseed := fmt.Sprintf("%v:%v:%v", c.account.AccAddress().String(), channel_st.PartB, c.denom)
	secret, hashcode := c.GenerateHashcode(randomseed)

	req := &node.MsgReqOpenChannel{
		Version:      c.version,
		PartA_Addr:   c.account.AccAddress().String(),
		PartB_Addr:   channel_st.PartB,
		PubkeyA:      c.account.PublicKey().String(),
		Deposit_Amt:  []uint64{5},
		Denom:        c.denom,
		Hashcode:     hashcode,
		PeerNodeAddr: ":50005",
		FirstSend:    []uint64{0},
		FirstRecv:    []uint64{0},
		Sequence:     1,
	}

	log.Println("RequestOpenChannel ...:", req)
	res, err := c.client.RequestOpenChannel(context.Background(), req)
	if err != nil {
		log.Println(err)
		return nil
	}

	confirmMsg := c.buildInfo(req, res, secret)

	log.Println("ConfirmOpenChannel ...")
	resConfirm, err := c.client.ConfirmOpenChannel(context.Background(), confirmMsg)
	if err != nil {
		log.Println(err)
	}

	log.Printf("res ConfirmOpenChannel...txhash %v, code %v", resConfirm.TxHash, resConfirm.Code)

	return nil
}

func eventHandler(c *MachineClient) {

	time.Sleep(3 * time.Second)

	msg := &node.Msg{
		Type: node.MsgType_REG_CHANNEL,
		Data: []byte(channel_st.ChannelID), // channelID
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
			log.Println("Make payment....")
			log.Println(rp)

			err := c.makeReqDirectPayment(rp)
			if err != nil {
				log.Println("makePayment err:", err.Error())
			}

			log.Println("BalanceA: ", comm_map[pre_commid].BalanceA)
			log.Println("BalanceB: ", comm_map[pre_commid].BalanceB)

			close(waitc)
			return
		//
		case node.MsgType_MSG_CLOSE:
			close(waitc)
			return
		//	log.Fatalf("Error: %v", data.Fields[field.Error].GetStringValue())

		default:
			close(waitc)
			//return status.Errorf(codes.Unimplemented, "Operation '%s' not implemented yet", operator)
			log.Println(codes.Unimplemented, "Operation '%s' not implemented yet ", msgtype)

			return
		}
	}
}

func (c *MachineClient) Register() (*node.MsgResUserRegister, error) {

	req := &node.MsgReqUserRegister{
		Pubkey:   c.account.PublicKey().String(),
		Addr:     c.account.AccAddress().String(),
		Sequence: 1,
	}

	return c.client.RequestUserRegister(context.Background(), req)
}

func (c *MachineClient) makeReqMultiHopPayment() error {

	_, hashcode := c.GenerateHashcode(channel_st.ChannelID)

	sendAmt := []uint64{1}
	recvAmt := []uint64{0}

	req := &node.MsgReqPayment{
		Type:       MULTI_HOP,
		SenderAddr: "", // addr of the sender node
		ChannelID:  channel_st.ChannelID,
		SendAmt:    sendAmt,
		RecvAmt:    recvAmt,
		Hashcode:   hashcode,
	}

	c.client.RequestPayment(context.Background(), req)

	return nil
}

func main() {

	var mmem = mmemonic[0]

	argsWithProg := os.Args
	if len(argsWithProg) >= 2 {
		i, _ := strconv.Atoi(os.Args[1])
		mmem = mmemonic[i]
	}

	log.Println("Mmem:", mmem)

	acc, err := account.NewAccount(common.COINTYPE).ImportAccount(mmem)
	if err != nil {
		log.Println("ImportAccount Err:", err.Error())
		return
	}

	client, stream, conn, err := connect(serverAddr)

	if err != nil {
		log.Printf("Err: %v", err)
		return
	}
	fmt.Println("Connect server done")

	c := NewMachineClient(stream, client, acc)

	c.Init()

	res, err := c.Register()
	if err != nil {
		log.Printf("Err: %v", err)
		return
	}

	fmt.Println("Register successful, res:", res)

	err = c.openChannel()
	if err != nil {
		log.Fatalf("Openchannel error: %v", err)
	}

	go eventHandler(c)

	for true {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your cmd:")
		cmd, _ := reader.ReadString('\n')

		switch cmd {
		case "r":

			fmt.Println("create a receipt")
		default:
			break
		}
	}
	//log.Println("Sleep 25s..")
	//time.Sleep(30 * time.Second)
	//log.Println("Start broadcast commiment..")
	//// SECTION: test broadcast a commitment
	////log.Println("Commitment to broadcast:", comm_map[pre_commid])

	//res, err := utils.BroadCastCommiment(*c.rpcClient, comm_map[pre_commid])
	//log.Println("BroadCastCommiment res code:", res.Code)
	////if err != nil && res.Code != 0 {
	////	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", res.TxHash, res.Code)
	////} else {
	////	log.Printf("BroadCastCommiment txhash %v with code: %v", res.TxHash, res.Code)
	////}
	//
	//// withdraw timelock
	//log.Println("Sleep 100s..")
	//time.Sleep(40 * time.Second)
	//log.Println("Start withdraw timelock..")
	//res1, txhash, err := utils.BuildAndBroadcastWithdrawTimeLockPartA(c.rpcClient, c.account, comm_map[pre_commid], &channel_st)
	//if err != nil {
	//	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", txhash, err.Error())
	//} else {
	//	log.Printf("BroadCastCommiment txhash %v with code: %v", res1.TxHash, res1.Code)
	//}

	<-waitc
	log.Println("Client close... ")
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("%v.CloseSend() got error %v, want %v", stream, err, nil)
	}

	conn.Close()
}

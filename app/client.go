package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/paymentnode/config"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/dungtt-astra/paymentnode/pkg/utils"
	node "github.com/dungtt-astra/paymentnode/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

var (
	serverAddr = flag.String("server_addr", "localhost:50005", "The server address in the format of host:port")
)

type Balance struct {
	partA float64
	partB float64
}

var balance_map = make(map[string]*Balance)

var mmemonic = []string{
	"call squirrel toddler country senior forum eternal aunt attract garbage soon decade attend tell box south visit learn lion come special simple spoon borrow",
	"run dry absent vicious since caution attitude elephant pool ocean sad entry chronic grunt alarm much human purse sausage stumble level very master fame",
	"draft eight argue sibling burden decade loop force walnut follow tunnel blossom elevator tank mutual hamster accident same primary year key loop doll keep",
	"skull drastic call search soda fiction benefit route motor tell miracle develop float priority mom run unique tree scrub intact visual club file hundred",
}

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
	rpcClient *client.Context
	client    node.NodeClient
	passcode  string
	timelock  uint64
}

//var cfg = &config.Config{
//	ChainId:       "astra_11110-1",
//	Endpoint:      "http://128.199.238.171:26657",
//	CoinType:      ethermintTypes.Bip44CoinType,
//	PrefixAddress: "astra",
//	TokenSymbol:   "aastra",
//}

var cfg = &config.Config{
	ChainId:       "testchain",
	Endpoint:      "http://localhost:26657",
	CoinType:      common.COINTYPE,
	PrefixAddress: "cosmos",
	TokenSymbol:   "stake",
}

func NewMachineClient(stream node.Node_OpenStreamClient,
	client node.NodeClient,
	account *account.PrivateKeySerialized) *MachineClient {

	return &MachineClient{
		stream,
		account,
		"stake",
		0,
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
	//c.denom = "stake"
	//c.amount = 0
	//c.version = "0.1"
	//c.timelock = uint64(common.TIMELOCK)

	// channel
	channel_st.PartB = "cosmos164xgenflr89l5q3q20e342z4ezpvyutlygaayf"

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
	ChannelID string
	SendAmt   uint64
	RecvAmt   uint64
	Nonce     uint64
	Hashcode  string
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

			err := c.makePayment(rp)
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
	log.Println("server pubkey:", res.Pubkey)
	peerPubkey, err := account.NewPKAccount(res.Pubkey)

	multisigAddr, multiSigPubkey, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(c.account.PublicKey(), peerPubkey.PublicKey(), 2)
	if err != nil {
		return common.Channel_st{}, err
	}

	channelID := fmt.Sprintf("%v:%v", multisigAddr, req.Denom)

	chann := common.Channel_st{
		ChannelID:       channelID,
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
	peerPubkey, err := account.NewPKAccount(res.Pubkey)

	multisigAddr, _, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(c.account.PublicKey(), peerPubkey.PublicKey(), 2)
	if err != nil {
		return nil
	}

	com := &common.Commitment_st{
		ChannelID:   fmt.Sprintf("%v:%v", multisigAddr, req.Denom),
		Denom:       req.Denom,
		BalanceA:    float64(req.Deposit_Amt + req.FirstRecv - req.FirstSend),
		BalanceB:    float64(res.Deposit_Amt - req.FirstRecv + req.FirstSend),
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

func (c *MachineClient) makePayment(rp NotifReqPaymentSt) error {

	//balance_map
	secret, hashcode := c.GenerateHashcode(rp.ChannelID)

	com := &common.Commitment_st{
		ChannelID:   rp.ChannelID,
		Denom:       c.denom,
		BalanceA:    balance_map[rp.ChannelID].partA + float64(rp.SendAmt) - float64(rp.RecvAmt),
		BalanceB:    balance_map[rp.ChannelID].partB + float64(rp.RecvAmt) - float64(rp.SendAmt),
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

	//log.Println("openChannel... channel_st:")
	//log.Println(channel_st)

	confirmMsg := c.buildConfirmMsg(cominfo, &channel_st)
	comm_map[comid].StrSigA = confirmMsg.CommitmentSig

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
		Deposit_Amt:  0,
		Denom:        c.denom,
		Hashcode:     hashcode,
		PeerNodeAddr: ":50005",
		FirstSend:    0,
		FirstRecv:    1,
	}

	log.Println("RequestOpenChannel ...")
	//log.Println("RequestOpenChannel ...:", req)
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

	err = c.openChannel()
	if err != nil {
		log.Fatalf("Openchannel error: %v", err)
	}

	go eventHandler(c)

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

package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/paymentnode/config"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/dungtt-astra/paymentnode/pkg/utils"
	node "github.com/dungtt-astra/paymentnode/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	serverAddr = flag.String("server_addr", "localhost:50005", "The server address in the format of host:port")
)

type Balance struct {
	partA float64
	partB float64
}

var balance_map = make(map[string]*Balance)

// var mmemonic = "blanket drama finish rally panic wool rich blush document lake friend hole treat random advice minute unique benefit live icon decline put icon vintage"
// var mmemonicBob = "embrace maid pond garbage almost crash silent maximum talent athlete view head horror label view sand ten market motion ceiling piano knee fun mechanic"
// var partBAddr = "cosmos1sd73jqkg2d7nfxefnr9psnl7tfxsy3pnltxfa7"

var mmemonic = "globe output invite salon drama fresh bleak nasty apology attack leaf height gravity bullet dolphin taxi must language state electric uncover damage danger auction"
var mmemonicBob = "walnut jazz because arena defense gadget demand before bleak shiver note glove history flee quarter sustain alcohol weather exact small violin diamond bean trumpet"
var partBAddr = "cosmos1mzh8m9yn90lt4dtwkq5grpcskqgmgzxmghdk72"

//var partBAddr = "cosmos164xgenflr89l5q3q20e342z4ezpvyutlygaayf"
//var mmemonic = "draft eight argue sibling burden decade loop force walnut follow tunnel blossom elevator tank mutual hamster accident same primary year key loop doll keep"
//var mmemonicBob = "opera buyer enact elbow taxi blur clap swap rigid loud paper planet use shrug core tell device silent stomach stage green have monkey evolve"

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
	channel_st.PartB = partBAddr

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

func (c *MachineClient) buildCommitmentInfo(channelID string, req *node.MsgReqOpenChannel, res *node.MsgResOpenChannel, secret string) *common.Commitment_st {
	com := &common.Commitment_st{
		ChannelID:   channelID,
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

func (c *MachineClient) openChannel() error {

	//pubkey := c.account.PublicKey().String()
	//log.Println("PartA addr:", c.account.AccAddress().String()) //client
	//log.Println("PartB addr:", channel_st.PartB) // node

	accBob, err := account.NewAccount(common.COINTYPE).ImportAccount(mmemonicBob)
	if err != nil {
		log.Println("ImportAccount Err:", err.Error())
		return err
	}

	multisigAddr, _, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(c.account.PublicKey(), accBob.PublicKey(), 2)
	if err != nil {
		return err
	}

	channelID := fmt.Sprintf("%v:%v", multisigAddr, c.denom)
	fmt.Println("channelID: ", channelID)
	//channelID := channel_st.ChannelID
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
	log.Println("RequestOpenChannel ...:", req)
	res, err := c.client.RequestOpenChannel(context.Background(), req)
	if err != nil {
		log.Println(err)
		return nil
	}

	channel_st, err = c.buildChannelInfo(req, res)
	if err != nil {
		log.Println(err)
	}

	cominfo := c.buildCommitmentInfo(channel_st.ChannelID, req, res, secret)

	comid := fmt.Sprintf("%v:%v", channel_st.ChannelID, cominfo.Nonce)
	comm_map[comid] = cominfo
	pre_commid = comid
	balance_map[cominfo.ChannelID] = &Balance{
		cominfo.BalanceA,
		cominfo.BalanceB,
	}

	// todo build confirm Msg

	//log.Println("openChannel... channel_st:")
	//log.Println(channel_st)

	confirmMsg := c.buildConfirmMsg(cominfo, &channel_st)
	comm_map[comid].StrSigA = confirmMsg.CommitmentSig

	txbyte, err := utils.PartABuildFullCommiment(c.rpcClient, c.GetAccount(), &channel_st, comm_map[comid])
	if err != nil {
		log.Println("PartABuildFullCommiment err: ", err)
		panic(err)
	}

	comm_map[comid].TxByteForBroadcast = txbyte

	//res1, err := c.rpcClient.BroadcastTx(txbyte)
	//if err != nil {
	//	log.Printf("\nBroadcast commit failed %v code %v", res1.TxHash, res1.Code)
	//} else {
	//	log.Printf("\nBroadcast commit ok  %v code %v", res1.TxHash, res1.Code)
	//}

	log.Println("ConfirmOpenChannel ...")
	resConfirm, err := c.client.ConfirmOpenChannel(context.Background(), confirmMsg)
	if err != nil {
		log.Println(err)
	}

	log.Printf("res ConfirmOpenChannel...txhash %v, code %v", resConfirm.TxHash, resConfirm.Code)

	return nil
}

func main() {

	acc, err := account.NewAccount(common.COINTYPE).ImportAccount(mmemonic)
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

	accBob, err := account.NewAccount(common.COINTYPE).ImportAccount(mmemonicBob)
	if err != nil {
		log.Println("ImportAccount Err:", err.Error())
		return
	}

	c.Init()

	go eventHandler(c)

	err = c.openChannel()
	if err != nil {
		log.Fatalf("Openchannel error: %v", err)
	}

	log.Println("Sleep 30s..")
	time.Sleep(30 * time.Second)
	///////////////////////////////////////////////////
	//  SENDER COMMITMENT
	///////////////////////////////////////////////////

	var comm = &common.SenderCommitment_st{
		ChannelID:          comm_map[pre_commid].ChannelID,
		Denom:              comm_map[pre_commid].Denom,
		BalanceA:           1,
		BalanceB:           3,
		AmountSendToC:      1, // todo amount send to C
		HashcodeA:          comm_map[pre_commid].HashcodeA,
		HashcodeB:          comm_map[pre_commid].HashcodeB,
		HashcodeC:          "secret", // todo hash invoice
		SecretA:            comm_map[pre_commid].SecretA,
		SecretB:            comm_map[pre_commid].SecretB,
		StrSigA:            comm_map[pre_commid].StrSigA,
		StrSigB:            comm_map[pre_commid].StrSigB,
		TxByteForBroadcast: comm_map[pre_commid].TxByteForBroadcast,
		PenaltyA_Tx:        comm_map[pre_commid].PenaltyA_Tx,
		PenaltyB_Tx:        comm_map[pre_commid].PenaltyB_Tx,
		Timelock:           2,
		Nonce:              comm_map[pre_commid].Nonce,
	}
	fmt.Println("timelock: ", comm.Timelock)

	// PART A create and sign Sender Commitment
	scommA, strsig1, err := utils.BuildAndSignSenderCommitmentPartA(c.rpcClient, c.account, comm, &channel_st)
	_ = scommA
	if err != nil {
		log.Println("BuildAndSignSenderCommitment err:", err.Error())
	}

	//log.Println("SenderCommitA:", scommA)
	//log.Println("Signature1:", strsig1)

	// PART B sign
	scommB, strsig2, err := utils.BuildAndSignSenderCommitmentPartA(c.rpcClient, accBob, comm, &channel_st)

	if err != nil {
		log.Println("BuildAndSignSenderCommitmentPartA err:", err.Error())
	}

	// PART B creates and sign Sender Commitment
	// scommB, strsig2, err := utils.BuildAndSignSenderCommitmentPartB(c.rpcClient, accBob, comm, &channel_st)

	// if err != nil {
	// 	log.Println("BuildAndSignSenderCommitment err:", err.Error())
	// }

	// Build & broadcast Sender Commitment
	log.Println("Start broadcast sender commiment..")
	txbyte, err := utils.BuildMultisigMsgReadyForBroadcast(c.rpcClient, channel_st.Multisig_Pubkey, strsig1, strsig2, scommB)
	if err != nil {
		log.Println("BuildCommitmentMsgReadyForBroadcast Err: ", err.Error())
		return
	}

	res, err := utils.BroadCastSignedTx(*c.rpcClient, txbyte)
	//res, err := utils.BuildAndBroadCastMultisigMsg(c.rpcClient, channel_st.Multisig_Pubkey, strsig1, strsig2, scommB)
	if err != nil {
		log.Println("BroadCastSignedTx err:", err.Error())
	}
	log.Println("BroadCastSignedTx res:", res)

	//log.Println("SenderCommitB:", scommB)
	//log.Println("Signature2:", strsig2)

	///////////////////////////////////////////////////
	//  RECEIVE COMMITMENT
	///////////////////////////////////////////////////
	//var r_comm = &common.ReceiveCommitment_st{
	//	ChannelID:          comm.ChannelID,
	//	Denom:              comm.Denom,
	//	BalanceA:           comm.BalanceA,
	//	BalanceB:           comm.BalanceB,
	//	AmountSendToC:      comm.AmountSendToC, // todo amount send to C
	//	HashcodeA:          comm.HashcodeA,
	//	HashcodeB:          comm.HashcodeB,
	//	HashcodeC:          comm.HashcodeC, // todo hash invoice
	//	SecretA:            comm.SecretA,
	//	SecretB:            comm.SecretB,
	//	StrSigA:            comm.StrSigA,
	//	StrSigB:            comm.StrSigB,
	//	TxByteForBroadcast: comm.TxByteForBroadcast,
	//	PenaltyA_Tx:        comm.PenaltyA_Tx,
	//	PenaltyB_Tx:        comm.PenaltyB_Tx,
	//	Timelock:           comm.Timelock,
	//	Nonce:              comm.Nonce,
	//}

	//rcomm, strsig2, err := utils.BuildAndSignReceiveCommitment(c.rpcClient, cBob.account, r_comm, &channel_st)
	//
	//if err != nil {
	//	log.Println("BuildAndSignSenderCommitment err:", err.Error())
	//}
	//
	//log.Println("ReceiveCommit:", rcomm)
	//log.Println("Signature:", strsig2)

	//log.Println("Start broadcast sender commiment..")
	//txbyte, err := utils.BuildMultisigMsgReadyForBroadcast(c.rpcClient, channel_st.Multisig_Pubkey, strsig1, strsig2, scommB)
	//if err != nil {
	//	log.Println("BuildCommitmentMsgReadyForBroadcast Err: ", err.Error())
	//	return
	//}
	//res, err := utils.BroadCastSignedTx(*c.rpcClient, txbyte)
	////res, err := utils.BuildAndBroadCastMultisigMsg(c.rpcClient, channel_st.Multisig_Pubkey, strsig1, strsig2, scommB)
	//if err != nil {
	//	log.Println("BroadCastSignedTx err:", err.Error())
	//}
	//log.Println("BroadCastSignedTx res:", res)

	//
	//
	//res, err := utils.BroadCastCommiment(*c.rpcClient, comm_map[pre_commid])
	//log.Println("BroadCastCommiment res:", res)
	//
	//
	// withdraw timelock
	log.Println("Sleep wait withdraw 1 time 2s..")
	time.Sleep(2 * time.Second)
	// log.Println("Start withdraw timelock..")
	// res1, txhash, err := utils.BuildAndBroadcastWithdrawTimeLockPartB(c.rpcClient, accBob, comm_map[pre_commid], &channel_st)
	// if err != nil {
	// 	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", txhash, err.Error())
	// } else {
	// 	log.Printf("BroadCastCommiment txhash %v with code: %v", res1.TxHash, res1.Code)
	// }

	// log.Println("Sleep wait withdraw 1.5 time 10s..")
	// time.Sleep(8 * time.Second)
	// log.Println("Start withdraw timelock..")
	// res1, txhash, err = utils.BuildAndBroadcastWithdrawTimeLockPartB(c.rpcClient, accBob, comm_map[pre_commid], &channel_st)
	// if err != nil {
	// 	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", txhash, err.Error())
	// } else {
	// 	log.Printf("BroadCastCommiment txhash %v with code: %v", res1.TxHash, res1.Code)
	// }

	log.Println("Start withdraw hashlock..")
	res1, txhash, err := utils.BuildAndBroadcastWithdrawHashLockPartA(c.rpcClient, c.account, comm_map[pre_commid], &channel_st)
	if err != nil {
		log.Fatalf("BuildAndBroadcastWithdrawHashLockPartA txhash %v failed with code: %v", txhash, err.Error())
	} else {
		log.Printf("BuildAndBroadcastWithdrawHashLockPartA txhash %v with code: %v", res1.TxHash, res1.Code)
	}

	// log.Println("Sleep wait withdraw 1.5 time 10s..")
	// time.Sleep(8 * time.Second)
	// log.Println("Start withdraw timelock..")
	// res1, txhash, err = utils.BuildAndBroadcastWithdrawHashLockPartA(c.rpcClient, c.account, comm_map[pre_commid], &channel_st)
	// if err != nil {
	// 	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", txhash, err.Error())
	// } else {
	// 	log.Printf("BroadCastCommiment txhash %v with code: %v", res1.TxHash, res1.Code)
	// }

	// log.Println("Sleep wait withdraw 2 time 20s..")
	// time.Sleep(10 * time.Second)
	// log.Println("Start withdraw timelock..")
	// res1, txhash, err = utils.BuildAndBroadcastWithdrawTimeLockPartB(c.rpcClient, accBob, comm_map[pre_commid], &channel_st)
	// if err != nil {
	// 	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", txhash, err.Error())
	// } else {
	// 	log.Printf("BroadCastCommiment txhash %v with code: %v", res1.TxHash, res1.Code)
	// }

	<-waitc
	log.Println("Client close... ")
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("%v.CloseSend() got error %v, want %v", stream, err, nil)
	}

	conn.Close()
}

package paymentnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dungtt-astra/astra-go-sdk/account"
	channelTypes "github.com/dungtt-astra/channel/x/channel/types"
	"github.com/dungtt-astra/paymentnode/config"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/dungtt-astra/paymentnode/pkg/user"
	"github.com/dungtt-astra/paymentnode/pkg/utils"
	node "github.com/dungtt-astra/paymentnode/proto"
	"google.golang.org/grpc"
)

var channel_map = make(map[string]*common.Channel_st)

var commitment_map = make(map[string]*common.Commitment_st)

type openchann_info struct {
	openchannel_msg       *channelTypes.MsgOpenChannel
	openchannel_sig_partB string
}

type Balance struct {
	partA     float64
	partB     float64
	preSecret string
	secret    string
}

var balance_map = make(map[string]Balance)

var openchanninfo_map = make(map[string]*openchann_info)

var nonce = uint64(0)

var g_channelid string
var gas_price = uint64(25)
var pre_commid string

type Node struct {
	node.UnimplementedNodeServer
	//stream      node.Node_ExecuteServer
	//cn          *channel.Channel
	//channelInfo data.Msg_Channel
	rpcClient *client.Context
	owner     *user.User
	address   string
}

func (n *Node) Start(args []string) {

	// create listener
	tcp := "tcp"
	address := ":50005"
	tokenSymbol := "stake"
	var mmemonic = "embrace maid pond garbage almost crash silent maximum talent athlete view head horror label view sand ten market motion ceiling piano knee fun mechanic"
	//var cfg = config.Config{
	//	ChainId:       "astra_11110-1",
	//	Endpoint:      "http://128.199.238.171:26657",
	//	CoinType:      ethermintTypes.Bip44CoinType,
	//	PrefixAddress: "astra",
	//	TokenSymbol:   "aastra",
	//	NodeAddr:      ":50005",
	//	Tcp:           "tcp",
	//}

	var cfg = &config.Config{
		ChainId:       "testchain",
		Endpoint:      "http://localhost:26657",
		CoinType:      common.COINTYPE,
		PrefixAddress: "cosmos",
		TokenSymbol:   "stake",
		NodeAddr:      ":50005",
		Tcp:           "tcp",
	}

	if len(args) >= 2 {
		n.owner.Passcode = args[2]
		address = fmt.Sprintf(":%v", args[1])
	}

	lis, err := net.Listen(tcp, address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	owner, err := user.NewUser("passcode", tokenSymbol, 5, mmemonic)
	if err != nil {
		panic(err.Error())
	}

	// create grpc server
	s := grpc.NewServer()
	node.RegisterNodeServer(s, &Node{
		rpcClient: utils.NewRpcClient(cfg),
		owner:     owner,
		address:   address,
	})

	// and start...
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (n *Node) isThisNode(addr string) bool {

	if addr != n.address {
		return false
	}

	return true
}

func getNonce() uint64 {

	var mu sync.Mutex
	mu.Lock()
	nonce++
	mu.Unlock()

	return nonce
}

func (n *Node) NewCommitment() common.Commitment_st {

	com := common.Commitment_st{
		ChannelID: "",
		Nonce:     getNonce(),
	}

	return com
}

func (n *Node) doReplyOpenChannel(req *node.MsgReqOpenChannel, cn *common.Channel_st) (*node.MsgResOpenChannel, error) {

	com_nonce := getNonce()

	channelID := fmt.Sprintf("%v:%v:%v", req.PartA_Addr, req.PartB_Addr, req.Denom)
	commitID := fmt.Sprintf("%v:%v", channelID, com_nonce)

	secret, hashcode := n.owner.GenerateHashcode(commitID)

	comm := common.Commitment_st{
		ChannelID:   channelID,
		Denom:       req.Denom,
		BalanceA:    float64(req.Deposit_Amt + req.FirstRecv - req.FirstSend),
		BalanceB:    n.owner.Deposit_Amt - float64(req.FirstRecv) + float64(req.FirstSend),
		HashcodeA:   req.Hashcode,
		HashcodeB:   hashcode,
		SecretA:     "",
		SecretB:     secret,
		PenaltyA_Tx: "", // if this commitment is invalidated, broadcast this to fire the cheating peer in case.
		PenaltyB_Tx: "",
		Timelock:    cn.Timelock,
		Nonce:       com_nonce,
	}

	//log.Println("comm:", comm)
	g_channelid = channelID
	balance_map[channelID] = Balance{
		comm.BalanceA,
		comm.BalanceB,
		secret,
		secret,
	}
	//balance_map[channelID].partB = comm.BalanceB
	//balance_map[channelID].secret = secret
	//balance_map[channelID].preSecret = secret

	_, str_sig, err := utils.BuildAndSignCommitmentMsgPartB(n.rpcClient, n.owner.Account, &comm, cn)
	if err != nil {
		return nil, err
	}

	//log.Println("First commitment msg:", ocmsg)
	//log.Println("Commitment Sig:", str_sig)

	comm.StrSigB = str_sig
	commitment_map[commitID] = &comm
	pre_commid = commitID

	res := &node.MsgResOpenChannel{
		Pubkey:         cn.PubkeyB.String(),
		Deposit_Amt:    uint64(n.owner.Deposit_Amt),
		Denom:          n.owner.Denom,
		Hashcode:       hashcode,
		Commitment_Sig: str_sig,
		Nonce:          com_nonce,
	}

	return res, nil
}

func (n *Node) parseToChannelSt(req *node.MsgReqOpenChannel) (*common.Channel_st, error) {

	peerPubkey, err := account.NewPKAccount(req.PubkeyA)

	multisigAddr, multiSigPubkey, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(peerPubkey.PublicKey(), n.owner.Account.PublicKey(), 2)
	if err != nil {
		return nil, err
	}

	log.Println("MultisigAddr:", multisigAddr)

	channelID := fmt.Sprintf("%v:%v:%v", req.PartA_Addr, req.PartB_Addr, req.Denom)
	chann := &common.Channel_st{
		ChannelID:       channelID,
		Multisig_Addr:   multisigAddr,
		Multisig_Pubkey: multiSigPubkey,
		PartA:           req.PartA_Addr,
		PartB:           req.PartB_Addr,
		PubkeyA:         peerPubkey.PublicKey(),
		PubkeyB:         n.owner.GetPubkey(),
		Denom:           req.Denom,
		Amount_partA:    float64(req.Deposit_Amt),
		Amount_partB:    n.owner.Deposit_Amt,
		Timelock:        uint64(common.TIMELOCK),
	}
	return chann, nil
}

func (n *Node) handleRequestOpenChannel(req *node.MsgReqOpenChannel) (*node.MsgResOpenChannel, error) {

	log.Println("PartA addr:", req.PartA_Addr)           // client
	log.Println("PartB addr:", n.owner.GetAccountAddr()) //node

	if n.isThisNode(req.PeerNodeAddr) {

		chann, err := n.parseToChannelSt(req)
		channel_map[chann.ChannelID] = chann

		res, err := n.doReplyOpenChannel(req, chann)
		if err != nil {
			return nil, err
		}

		return res, nil

	} else {
		// todo connect to other node

		res := &node.MsgResOpenChannel{
			Pubkey: "node pubkey hello",
		}

		return res, nil
	}
}

func (n *Node) validateRequestOpenChannel(req *node.MsgReqOpenChannel) error {

	if len(req.Denom) == 0 {
		return errors.New("Invalid denom")
	}

	_, err := sdk.AccAddressFromBech32(req.PartA_Addr)
	if err != nil {
		return err
	}

	_, err = sdk.AccAddressFromBech32(req.PartB_Addr)
	if err != nil {
		return err
	}

	return nil
}

func (n *Node) RequestOpenChannel(ctx context.Context, req *node.MsgReqOpenChannel) (*node.MsgResOpenChannel, error) {

	log.Println("RequestOpenChannel received...")
	log.Println("RequestOpenChannel:", req)
	err := n.validateRequestOpenChannel(req)
	if err != nil {
		return nil, err
	}

	return n.handleRequestOpenChannel(req)

	//status.Errorf(codes.Unimplemented, "method RequestOpenChannel not implemented")
}

func (n *Node) handleConfirmOpenChannel(msg *node.MsgConfirmOpenChannel) (*sdk.TxResponse, error) {
	if msg.Type == node.MsgType_ERROR {
		log.Println(msg.CommitmentSig)
		return nil, errors.New("Client reject openchannel")
	}

	commitment_map[pre_commid].StrSigA = msg.GetCommitmentSig()
	comm := commitment_map[pre_commid]
	chann := channel_map[msg.ChannelID]

	// broadcast commitment
	txbyte, err := utils.PartBBuildFullCommiment(n.rpcClient, n.owner.Account, chann, comm)
	if err != nil {
		log.Println("PartBBuildFullCommiment er:", err.Error())
	}

	commitment_map[pre_commid].TxByteForBroadcast = txbyte

	openChannelRequest, partBsig, err := utils.BuildAndSignOpenChannelMsg(n.rpcClient, n.owner.Account, chann)
	if err != nil {
		return nil, err
	}
	//log.Println("openChannelRequest:", openChannelRequest)
	//log.Println("PartB OC sig:", partBsig)

	txResponse, err := utils.BuildAndBroadCastMultisigMsg(n.rpcClient, chann.Multisig_Pubkey, msg.OpenChannelTxSig, partBsig, openChannelRequest)
	if err != nil {
		log.Printf("handleConfirmOpenChannel:BuildAndBroadCastMultisigMsg Err: %v", err.Error())
		return nil, err
	}

	log.Printf("txhash: %v, code: %v \n", txResponse.TxHash, txResponse.Code)

	return txResponse, nil
}

func (n *Node) validateConfirmOpenChannel(msg *node.MsgConfirmOpenChannel) error {
	// todo validateConfirmOpenChannel
	return nil
}

func (n *Node) ConfirmOpenChannel(ctx context.Context, msg *node.MsgConfirmOpenChannel) (*node.MsgResConfirmOpenChannel, error) {

	log.Println("ConfirmOpenChannel receive...") //, msg)

	if err := n.validateConfirmOpenChannel(msg); err != nil {
		return nil, err
	}

	txResponse, err := n.handleConfirmOpenChannel(msg)
	if err != nil {
		log.Printf("ConfirmOpenChannel Err: %v", err.Error())
		return nil, err
	}

	txfee := uint64(txResponse.GasUsed) * gas_price

	resmsg := &node.MsgResConfirmOpenChannel{
		Code:   txResponse.Code,
		TxHash: txResponse.TxHash,
		Data:   txResponse.Data,
		TxFee:  txfee,
	}

	log.Println("Balance A:", balance_map[msg.ChannelID].partA)
	log.Println("Balance B:", balance_map[msg.ChannelID].partB)

	return resmsg, nil
	//return nil, status.Errorf(codes.Unimplemented, "meresd ConfirmOpenChannel not implemented")
}

type NotifReqPaymentSt struct {
	ChannelID string
	SendAmt   uint64
	RecvAmt   uint64
	Nonce     uint64
	Hashcode  string
}

func (n *Node) NotifyPayment(channelID string) error {

	log.Println("NotifyPayment")

	tnonce := getNonce()
	stream := stream_map[channelID]
	commitID := fmt.Sprintf("%v:%v", channelID, tnonce)
	secret, hashcode := n.owner.GenerateHashcode(commitID)

	rp := NotifReqPaymentSt{
		ChannelID: channelID,
		SendAmt:   1,
		RecvAmt:   0,
		Nonce:     tnonce,
		Hashcode:  hashcode,
	}

	comm := &common.Commitment_st{
		ChannelID:   channelID,
		Denom:       n.owner.Denom,
		BalanceA:    balance_map[channelID].partA + float64(rp.SendAmt) - float64(rp.RecvAmt),
		BalanceB:    balance_map[channelID].partB + float64(rp.RecvAmt) - float64(rp.SendAmt),
		HashcodeA:   "",
		HashcodeB:   hashcode,
		SecretA:     "",
		SecretB:     secret,
		PenaltyA_Tx: "",
		PenaltyB_Tx: "",
		Timelock:    uint64(common.TIMELOCK),
		Nonce:       tnonce,
	}

	commid := fmt.Sprintf("%v:%v", comm.ChannelID, tnonce)
	commitment_map[commid] = comm

	data, _ := json.Marshal(rp)
	msg := &node.Msg{
		Type: node.MsgType_REQ_PAYMENT,
		Data: data,
	}
	stream.Send(msg)

	return nil
}

func (n *Node) ConfirmPayment(ctx context.Context, msg *node.MsgConfirmPayment) (*node.MsgResConfirmPayment, error) {

	log.Println("ConfirmPayment:", msg)

	commitment_map[msg.CommID].SecretA = msg.SecretPreComm

	log.Println("Balance A:", balance_map[msg.ChannelID].partA)
	log.Println("Balance B:", balance_map[msg.ChannelID].partB)
	//
	//closemsg := &node.Msg{
	//	Type: node.MsgType_MSG_CLOSE,
	//	Data: []byte("Request close"),
	//}
	//err := stream_map[msg.ChannelID].Send(closemsg)
	//log.Println("BuildAndSignCommitmentMsg: err:", err.Error())
	res := &node.MsgResConfirmPayment{
		ChannelID: msg.ChannelID,
	}

	return res, nil
}

func (n *Node) RequestPayment(ctx context.Context, msg *node.MsgReqPayment) (*node.MsgResPayment, error) {

	log.Println("RequestPayment: ", msg)
	comm := commitment_map[msg.CommitmentID]
	if comm == nil {
		return nil, errors.New("Wrong commitment ID")
	}

	comm.HashcodeA = msg.Hashcode
	commitment_map[msg.CommitmentID] = comm

	_, com_sig, err := utils.BuildAndSignCommitmentMsgPartB(n.rpcClient, n.owner.Account, comm, channel_map[msg.ChannelID])
	if err != nil {
		return nil, err
	}

	res := &node.MsgResPayment{
		ChannelID:     msg.ChannelID,
		CommitmentID:  msg.CommitmentID,
		CommitmentSig: com_sig,
		SecretPreComm: balance_map[msg.ChannelID].preSecret,
	}

	balance_map[msg.ChannelID] = Balance{comm.BalanceA,
		comm.BalanceB,
		balance_map[msg.ChannelID].secret,
		comm.SecretB,
	}

	return res, nil
}

var stream_map = make(map[string]node.Node_OpenStreamServer)

func (n *Node) OpenStream(stream node.Node_OpenStreamServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Println("EOF; End of stream")
			return nil
		}
		if err != nil {
			return err
		}

		msgType := msg.GetType()
		msgData := msg.GetData()

		switch msgType {
		case node.MsgType_REG_CHANNEL:
			stream_map[string(msgData)] = stream
			//n.NotifyPayment(g_channelid)
			//stream.Context().
		default:

		}
		log.Printf("Received Msg type: %v, RawData: %+v\n", msgType, string(msgData))

	}
}

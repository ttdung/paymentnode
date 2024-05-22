package paymentnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ttdung/astra-go-sdk/account"
	channelTypes "github.com/ttdung/channel_v0.46/x/channel/types"
	"github.com/ttdung/paymentnode/config"
	"github.com/ttdung/paymentnode/pkg/common"
	"github.com/ttdung/paymentnode/pkg/user"
	"github.com/ttdung/paymentnode/pkg/utils"
	node "github.com/ttdung/paymentnode/proto"
	"google.golang.org/grpc"
	//"google.golang.org/grpc"
	"io"
	"log"
	"math/big"
	"net"
	"sync"
	"time"
)

var channel_map = make(map[string]*common.Channel_st)

var comm_map = make(map[string]*common.Commitment_st)

var user_channel_map = make(map[string]string)

type openchann_info struct {
	openchannel_msg       *channelTypes.MsgOpenchannel
	openchannel_sig_partB string
}

const SINGLE_HOP = 0
const MULTI_HOP = 1

type Balance struct {
	partA     []uint64
	partB     []uint64
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

	var mmemonic = "rich cost ordinary claim citizen battle expose popular glad enlist hint knock load across blade portion chaos type slush online memory hunt drama clarify"

	//tokenSymbol := "aastra"
	//var cfg = config.Config{
	//	ChainId:       "astra_11115-1",
	//	Endpoint:      "http://localhost:26657",
	//	CoinType:      common.COINTYPE, //ethermintTypes.Bip44CoinType,
	//	PrefixAddress: "astra",
	//	TokenSymbol:   []string{common.DENOM},
	//	NodeAddr:      ":50005",
	//	Tcp:           "tcp",
	//}

	var cfg = &config.Config{
		ChainId:       "channel_v0.46",
		Endpoint:      "http://localhost:26657",
		CoinType:      common.COINTYPE,
		PrefixAddress: "cosmos",
		TokenSymbol:   []string{"stake"},
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

	owner, err := user.NewUser("passcode", cfg.TokenSymbol, []uint64{5}, mmemonic)
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

func (n *Node) RequestUserRegister(ctx context.Context, in *node.MsgReqUserRegister) (*node.MsgResUserRegister, error) {

	log.Println("RequestRequestUserRegisterOpenChannel received...")
	log.Println("RequestOpenChannel:", in)
	peerPubkey, err := account.NewPKAccount(in.Pubkey)

	multisigAddr, _, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(n.owner.Account.PublicKey(), peerPubkey.PublicKey(), 2)
	if err != nil {
		return nil, err
	}

	channelID := fmt.Sprintf("%v:%v", multisigAddr, in.Sequence)

	res := &node.MsgResUserRegister{
		Result:    true,
		Pubkey:    n.owner.Account.PublicKey().String(),
		ChannelID: channelID,
	}

	user_channel_map[in.Addr] = channelID

	return res, nil
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

	channelID := fmt.Sprintf("%v:%v", cn.Multisig_Addr, req.Sequence)
	commitID := fmt.Sprintf("%v:%v", channelID, com_nonce)

	secret, hashcode := n.owner.GenerateHashcode(commitID)

	var balanceA, balanceB []uint64
	balanceA = make([]uint64, len(req.Deposit_Amt))
	balanceB = make([]uint64, len(req.Deposit_Amt))

	for i := 0; i < len(req.Deposit_Amt); i++ {
		balanceA[i] = req.Deposit_Amt[i] + req.FirstRecv[i] - req.FirstSend[i]
		balanceB[i] = n.owner.Deposit_Amt[i] - req.FirstRecv[i] + req.FirstSend[i]
	}

	comm := common.Commitment_st{
		ChannelID:   channelID,
		Denom:       req.Denom,
		BalanceA:    balanceA,
		BalanceB:    balanceB,
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
	comm_map[commitID] = &comm
	pre_commid = commitID

	res := &node.MsgResOpenChannel{
		Pubkey:         cn.PubkeyB.String(),
		Deposit_Amt:    n.owner.Deposit_Amt,
		Denom:          n.owner.Denom,
		Hashcode:       hashcode,
		Commitment_Sig: str_sig,
		Nonce:          com_nonce,
	}

	return res, nil
}

func (n *Node) parseToChannelSt(req *node.MsgReqOpenChannel) (*common.Channel_st, error) {
	log.Println("reqPubkeyA:", req.PubkeyA)
	peerPubkey, err := account.NewPKAccount(req.PubkeyA)
	log.Println("peerPubkey:", peerPubkey.PublicKey())
	log.Println("peerPubkey.Address:", peerPubkey.PublicKey().Address())
	//
	//log.Println("owner.PublicKey():", n.owner.Account.PublicKey())
	log.Println("owner.PublicKey() Address:", n.owner.Account.PublicKey().Address())

	multisigAddr, multiSigPubkey, err := account.NewAccount(common.COINTYPE).CreateMulSignAccountFromTwoAccount(n.owner.Account.PublicKey(), peerPubkey.PublicKey(), 2)
	if err != nil {
		return nil, err
	}

	log.Println("MultisigAddr:", multisigAddr)
	log.Println("multiSigPubkey:", multiSigPubkey.Address())

	channelID := fmt.Sprintf("%v:%v", multisigAddr, req.Sequence)

	chann := &common.Channel_st{
		ChannelID:       channelID,
		Multisig_Addr:   multisigAddr,
		Multisig_Pubkey: multiSigPubkey,
		PartA:           req.PartA_Addr,
		PartB:           req.PartB_Addr,
		PubkeyA:         peerPubkey.PublicKey(),
		PubkeyB:         n.owner.GetPubkey(),
		Denom:           req.Denom,
		Amount_partA:    req.Deposit_Amt,
		Amount_partB:    n.owner.Deposit_Amt,
		Timelock:        uint64(common.TIMELOCK),
	}
	log.Println("=========================chann:", chann)
	return chann, nil
}

func (n *Node) initNewMultisigAddr(chann *common.Channel_st) error {

	fmt.Println("Multisig_Addr:", chann.Multisig_Addr)
	fmt.Println("n.owner.Account mnemonic:", n.owner.Account.Mnemonic())
	res, err := utils.TransferTokenWithPrivateKey(n.owner.Account.PrivateKeyToString(),
		*n.rpcClient,
		chann.Denom[0],
		common.COINTYPE,
		chann.Multisig_Addr,
		big.NewInt(1),
		200000,
		common.GASPRICE,
	)

	if err != nil {
		return err
	}

	if res.Code != 0 { // not success
		return fmt.Errorf("Send failed with txhash %v & code %v", res.TxHash, res.Code)
	}

	return nil
}

func (n *Node) handleRequestOpenChannel(req *node.MsgReqOpenChannel) (*node.MsgResOpenChannel, error) {

	log.Println("PartA addr:", req.PartA_Addr)           // client
	log.Println("PartB addr:", n.owner.GetAccountAddr()) //node
	//log.Println("PartB mnemonic:", n.owner.Account.Mnemonic()) //node

	if n.isThisNode(req.PeerNodeAddr) {

		chann, err := n.parseToChannelSt(req)
		channel_map[chann.ChannelID] = chann

		// test if new multisig, balance = 0 then send some token to init new multisig addr
		balance, err := utils.GetBalance(n.rpcClient, chann.Multisig_Addr)
		if err != nil {
			return nil, err
		}

		if balance.Int64() == 0 {
			err = n.initNewMultisigAddr(chann)
			if err != nil {
				log.Println("handleRequestOpenChannel: ", err.Error())
				return nil, err
			} else {
				// Wait for this tx included in chain
				count := 0
				for true {
					balance, err := utils.GetBalance(n.rpcClient, chann.Multisig_Addr)

					if err != nil {
						return nil, err
					}
					time.Sleep(time.Second * 1)

					count = count + 1
					if count == 3 {
						break
					}
					if balance.Int64() == 0 {
						continue
					} else {
						break
					}
				}
			}
		}

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

	comm_map[pre_commid].StrSigA = msg.GetCommitmentSig()
	comm := comm_map[pre_commid]
	chann := channel_map[msg.ChannelID]

	// broadcast commitment
	txbyte, err := utils.PartBBuildFullCommiment(n.rpcClient, n.owner.Account, chann, comm)
	if err != nil {
		log.Println("PartBBuildFullCommiment er:", err.Error())
	}

	comm_map[pre_commid].TxByteForBroadcast = txbyte

	openChannelRequest, partBsig, err := utils.BuildAndSignOpenChannelMsg(n.rpcClient, n.owner.Account, chann)
	if err != nil {
		return nil, err
	}
	log.Println("Openchannel:BuildAndBroadCastMultisigMsg: openChannelRequest:", openChannelRequest)
	log.Println("Openchannel:chann.Multisig_Pubkey:", chann.Multisig_Pubkey)
	log.Println("Openchannel:PartA OC sig:", msg.OpenChannelTxSig)
	log.Println("Openchannel:PartB OC sig:", partBsig)

	txResponse, err := utils.BuildAndBroadCastMultisigMsg(n.rpcClient, chann.Multisig_Pubkey, msg.OpenChannelTxSig, partBsig, openChannelRequest)
	if err != nil {
		log.Printf("handleConfirmOpenChannel:BuildAndBroadCastMultisigMsg Err: %v", err.Error())
		return nil, err
	}

	log.Printf("txhash: %v, txResponse: %v \n", txResponse.TxHash, txResponse)

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
	SendAmt   []uint64
	RecvAmt   []uint64
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
		SendAmt:   []uint64{1},
		RecvAmt:   []uint64{0},
		Nonce:     tnonce,
		Hashcode:  hashcode,
	}

	var balanceA, balanceB []uint64
	balanceA = make([]uint64, len(balance_map[channelID].partA))
	balanceB = make([]uint64, len(balance_map[channelID].partA))

	for i := 0; i < len(balance_map[channelID].partA); i++ {
		balanceA[i] = balance_map[channelID].partA[i] + rp.SendAmt[i] - rp.RecvAmt[i]
		balanceB[i] = balance_map[channelID].partB[i] + rp.RecvAmt[i] - rp.SendAmt[i]
	}

	comm := &common.Commitment_st{
		ChannelID:   channelID,
		Denom:       n.owner.Denom,
		BalanceA:    balanceA,
		BalanceB:    balanceB,
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
	comm_map[commid] = comm

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

	comm_map[msg.CommID].SecretA = msg.SecretPreComm

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

func (n *Node) handleRequestMultiHopPayment(ctx context.Context, msg *node.MsgReqPayment) (*node.MsgResPayment, error) {

	// search sender
	channelId := user_channel_map[msg.SenderAddr]
	if len(channelId) < 1 {
		return nil, fmt.Errorf("Not found SenderAddr")
	}

	// Create commitment with sender

	return nil, nil
}

func (n *Node) handleRequestDirectPayment(ctx context.Context, msg *node.MsgReqPayment) (*node.MsgResPayment, error) {
	log.Println("RequestPayment: ", msg)
	comm := comm_map[msg.CommitmentID]
	if comm == nil {
		return nil, errors.New("Wrong commitment ID")
	}

	comm.HashcodeA = msg.Hashcode
	comm_map[msg.CommitmentID] = comm

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

func (n *Node) RequestPayment(ctx context.Context, msg *node.MsgReqPayment) (*node.MsgResPayment, error) {

	if msg.Type == SINGLE_HOP {
		return n.handleRequestDirectPayment(ctx, msg)
	} else if msg.Type == MULTI_HOP {
		return n.handleRequestMultiHopPayment(ctx, msg)
	}

	return nil, nil
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
			////n.NotifyPayment(g_channelid)
			////stream.Context().
			//
			//// withdraw hashlock
			//log.Println("Sleep 50s..")
			//time.Sleep(50 * time.Second)
			//log.Println("Start withdraw hashlock..")
			//secretA := "abcd"
			//
			//res1, txhash, err := utils.BuildAndBroadcastWithdrawHashLockPartB(n.rpcClient, n.owner.Account, comm_map[pre_commid], channel_map[comm_map[pre_commid].ChannelID], secretA)
			//if err != nil {
			//	log.Fatalf("BroadCastCommiment txhash %v failed with code: %v", txhash, err.Error())
			//} else {
			//	log.Printf("BroadCastCommiment txhash %v with code: %v", res1.TxHash, res1.Code)
			//}
		default:

		}
		log.Printf("Received Msg type: %v, RawData: %+v\n", msgType, string(msgData))

	}
}

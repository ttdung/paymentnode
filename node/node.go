package paymentnode

import (
	"context"
	"errors"
	"fmt"
	channelTypes "github.com/AstraProtocol/channel/x/channel/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/dungtt-astra/astra-go-sdk/account"
	"github.com/dungtt-astra/paymentnode/config"
	"github.com/dungtt-astra/paymentnode/pkg/common"
	"github.com/dungtt-astra/paymentnode/pkg/user"
	"github.com/dungtt-astra/paymentnode/pkg/utils"
	node "github.com/dungtt-astra/paymentnode/proto"
	"github.com/evmos/ethermint/app"
	"github.com/evmos/ethermint/encoding"
	ethermintTypes "github.com/evmos/ethermint/types"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"sync"
)

var TIMELOCK = 100

var channel_map = make(map[string]*common.Channel_st)

var commitment_map = make(map[string]*common.Commitment_st)

type openchann_info struct {
	openchannel_msg       *channelTypes.MsgOpenChannel
	openchannel_sig_partB string
}

var openchanninfo_map = make(map[string]*openchann_info)

var nonce = uint64(0)

type Node struct {
	node.UnimplementedNodeServer
	//stream      node.Node_ExecuteServer
	//cn          *channel.Channel
	//channelInfo data.Msg_Channel
	rpcClient client.Context
	Owner     *user.User
	address   string
}

func (n *Node) Start(args []string) {
	// create listener
	tcp := "tcp"
	address := ":50005"
	tokenSymbol := "aastra"
	var mmemonic = "leaf emerge will mix junior smile tortoise mystery scheme chair fancy afraid badge carpet pottery raw vicious hood exile amateur symbol battle oyster action"

	if len(args) >= 2 {
		n.Owner.Passcode = args[2]
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
		rpcClient: n.rpcClient,
		Owner:     owner,
		address:   address,
	})

	// and start...
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func NewNode(cfg *config.Config) *Node {
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetPurpose(44)
	sdkConfig.SetCoinType(ethermintTypes.Bip44CoinType) // Todo

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
		WithCodec(encodingConfig.Marshaler).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithChainID(cfg.ChainId).
		WithAccountRetriever(authTypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastSync).
		WithTxConfig(encodingConfig.TxConfig)

	return &Node{
		rpcClient: rpcClient,
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

	secret, hashcode := n.Owner.GenerateHashcode(commitID)

	comm := common.Commitment_st{
		ChannelID:   channelID,
		Denom:       req.Denom,
		BalanceA:    float64(req.Deposit_Amt + req.FirstRecv - req.FirstSend),
		BalanceB:    n.Owner.Deposit_Amt - float64(req.FirstRecv+req.FirstSend),
		HashcodeA:   req.Hashcode,
		HashcodeB:   hashcode,
		SecretA:     "",
		SecretB:     secret,
		PenaltyA_Tx: "", // if this commitment is invalidated, broadcast this to fire the cheating peer in case.
		PenaltyB_Tx: "",
		Timelock:    cn.Timelock,
		Nonce:       com_nonce,
	}

	_, str_sig, err := utils.BuildAndSignCommitmentMsg(n.rpcClient, n.Owner.Account, &comm, cn)
	if err != nil {
		return nil, err
	}

	comm.StrSigB = str_sig
	commitment_map[commitID] = &comm

	res := &node.MsgResOpenChannel{
		Pubkey:         cn.PubkeyB.String(),
		Deposit_Amt:    uint64(n.Owner.Deposit_Amt),
		Denom:          n.Owner.Denom,
		Hashcode:       hashcode,
		Commitment_Sig: str_sig,
		Nonce:          com_nonce,
	}

	return res, nil
}

func (n *Node) parseToChannelSt(req *node.MsgReqOpenChannel) (*common.Channel_st, error) {

	peerPubkey, err := account.NewPKAccount(req.PubkeyA)

	multisigAddr, multiSigPubkey, err := account.NewAccount(60).CreateMulSignAccountFromTwoAccount(peerPubkey.PublicKey(), n.Owner.Account.PublicKey(), 2)
	if err != nil {
		return nil, err
	}

	channelID := fmt.Sprintf("%v:%v:%v", req.PartA_Addr, req.PartB_Addr, req.Denom)
	chann := &common.Channel_st{
		Index:           channelID,
		Multisig_Addr:   multisigAddr,
		Multisig_Pubkey: multiSigPubkey,
		PartA:           req.PartA_Addr,
		PartB:           req.PartB_Addr,
		PubkeyA:         peerPubkey.PublicKey(),
		PubkeyB:         n.Owner.GetPubkey(),
		Denom:           req.Denom,
		Amount_partA:    float64(req.Deposit_Amt),
		Amount_partB:    n.Owner.Deposit_Amt,
		Timelock:        uint64(TIMELOCK),
	}
	return chann, nil
}

func (n *Node) handleRequestOpenChannel(req *node.MsgReqOpenChannel) (*node.MsgResOpenChannel, error) {

	log.Println("PartB addr:", n.Owner.GetAccountAddr())
	log.Println("PartA addr:", req.PartA_Addr)

	if n.isThisNode(req.PeerNodeAddr) {

		chann, err := n.parseToChannelSt(req)
		channel_map[chann.Index] = chann

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

	chann := channel_map[msg.ChannelID]

	openChannelRequest, partBsig, err := utils.BuildAndSignOpenChannelMsg(n.rpcClient, n.Owner.Account, chann)
	if err != nil {
		return nil, err
	}

	txResponse, err := utils.BuildAndBroadCastMultisigMsg(n.rpcClient, chann.Multisig_Pubkey, msg.OpenChannelTxSig, partBsig, openChannelRequest)
	if err != nil {
		log.Printf("BuildAndBroadCastMultisigMsg Err: %v", err.Error())
		return nil, err
	}

	return txResponse, nil
}

func (n *Node) ConfirmOpenChannel(ctx context.Context, msg *node.MsgConfirmOpenChannel) (*node.MsgResConfirmOpenChannel, error) {

	log.Println("ConfirmOpenChannel receive:", msg)

	txResponse, err := n.handleConfirmOpenChannel(msg)
	if err != nil {
		log.Printf("ConfirmOpenChannel Err: %v", err.Error())
		return nil, err
	}

	resmsg := &node.MsgResConfirmOpenChannel{
		Code:   txResponse.Code,
		TxHash: txResponse.TxHash,
		Data:   txResponse.Data,
	}

	return resmsg, nil
	//return nil, status.Errorf(codes.Unimplemented, "meresd ConfirmOpenChannel not implemented")
}

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

		log.Printf("Received Msg type: %v, RawData: %+v\n", msgType, string(msgData))

	}
}

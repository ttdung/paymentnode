package paymentnode

import (
	node "github.com/dungtt-astra/paymentnode/proto"
	"io"
	"log"
)

type Node struct {
	node.UnimplementedNodeServer
	//stream      node.Node_ExecuteServer
	//thisAccount *account.PrivateKeySerialized
	//cn          *channel.Channel
	//channelInfo data.Msg_Channel
	//rpcClient   client.Context
}

func (n *Node) OpenStream(stream proto.Node_OpenStreamServer) error {
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
		//resData := []byte{}
		//resType := proto.MsgType_CHAT
		//switch msgType {
		//case proto.MsgType_CHAT:
		//	resData = s.ProcessClientChat(msgData)
		//case proto.MsgType_REQUESTDATA:
		//	resType = proto.MsgType_RESPONSEDATA
		//	resData = s.ProcessClientRequest(msgData)
		//case proto.MsgType_RESPONSEDATA:
		//	resData = s.ProcessClientResponse(msgData)
		//default:
		//	return status.Errorf(codes.Unimplemented, "Message Type '%s' not implemented yet", msgType)
		//}
		//if len(resData) != 0 {
		//	if err := stream.Send(&proto.ServerMsg{
		//		Type: resType,
		//		Data: resData,
		//	}); err != nil {
		//		return err
		//	}
		//}
	}
}

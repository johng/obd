package lightclient

import (
	"LightningOnOmni/bean"
	"LightningOnOmni/bean/enum"
	"LightningOnOmni/rpc"
	"LightningOnOmni/service"
	"LightningOnOmni/tool"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

func (client *Client) Write() {
	defer func() {
		e := client.Socket.Close()
		if e != nil {
			log.Println(e)
		} else {
			log.Println("socket closed after writing...")
		}
	}()

	for {
		select {
		case data, ok := <-client.SendChannel:
			if !ok {
				_ = client.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			log.Println("send data", string(data))
			_ = client.Socket.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func (client *Client) Read() {
	defer func() {
		_ = service.UserService.UserLogout(client.User)
		if client.User != nil {
			delete(GlobalWsClientManager.OnlineUserMap, client.User.PeerId)
			delete(service.OnlineUserMap, client.User.PeerId)
		}
		GlobalWsClientManager.Disconnected <- client
		_ = client.Socket.Close()
		log.Println("socket closed after reading...")
	}()

	for {
		_, dataReq, err := client.Socket.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}

		var msg bean.RequestMessage
		log.Println("request data: ", string(dataReq))
		parse := gjson.Parse(string(dataReq))

		if parse.Exists() == false {
			log.Println("wrong json input")
			client.sendToMyself(enum.MsgType_Error, false, string(dataReq))
			continue
		}

		msg.Type = enum.MsgType(parse.Get("type").Int())
		msg.Data = parse.Get("data").String()
		msg.SenderPeerId = parse.Get("sender_peer_id").String()
		msg.RecipientPeerId = parse.Get("recipient_peer_id").String()
		msg.PubKey = parse.Get("pub_key").String()
		msg.Signature = parse.Get("signature").String()

		// check the Recipient is online
		if tool.CheckIsString(&msg.RecipientPeerId) {
			_, err := client.FindUser(&msg.RecipientPeerId)
			if err != nil {
				client.sendToMyself(msg.Type, true, "can not find target user")
				continue
			}
		}

		// check the data whether is right signature
		if tool.CheckIsString(&msg.PubKey) && tool.CheckIsString(&msg.Signature) {
			rpcClient := rpc.NewClient()
			result, err := rpcClient.VerifyMessage(msg.PubKey, msg.Signature, msg.Data)
			if err != nil {
				client.sendToMyself(msg.Type, false, err.Error())
				continue
			}
			if gjson.Parse(result).Bool() == false {
				client.sendToMyself(msg.Type, false, "error signature")
				continue
			}
		}

		var sendType = enum.SendTargetType_SendToNone
		status := false
		var dataOut []byte
		var needLogin = true
		if msg.Type < 1000 && msg.Type >= 0 {
			sendType, dataOut, status = client.userModule(msg)
			needLogin = false
		}

		if msg.Type > 1000 {
			sendType, dataOut, status = client.omniCoreModule(msg)
			needLogin = false
		}

		if needLogin {
			//not login
			if client.User == nil {
				client.sendToMyself(msg.Type, false, "please login")
				continue
			} else { // already login
				for {
					typeStr := strconv.Itoa(int(msg.Type))
					//-32 -3201 -3202 -3203 -3204
					if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_ChannelOpen_N32))) {
						sendType, dataOut, status = client.channelModule(msg)
						break
					}
					//-33 -3301 -3302 -3303 -3304
					if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_ChannelAccept_N33))) {
						sendType, dataOut, status = client.channelModule(msg)
						break
					}
					//-34 -3400 -3401 -3402 -3403 -3404
					if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_FundingCreate_OmniCreate_N34))) {
						sendType, dataOut, status = client.fundingTransactionModule(msg)
						break
					}

					//-35 -3500
					if msg.Type == enum.MsgType_FundingSign_OmniSign_N35 ||
						msg.Type == enum.MsgType_FundingSign_BtcSign_N3500 {
						sendType, dataOut, status = client.fundingSignModule(msg)
						break
					}

					if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_FundingSign_OmniSign_N35))) {
						//-351 -35101 -35102 -35103 -35104
						if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_CommitmentTx_Create_N351))) {
							sendType, dataOut, status = client.commitmentTxModule(msg)
							break
						}
						//-352 -35201 -35202 -35203 -35204
						if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_CommitmentTxSigned_Sign_N352))) {
							sendType, dataOut, status = client.commitmentTxSignModule(msg)
							break
						}
						//-353 -35301 -35302 -35303 -35304
						if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_GetBalanceRequest_N353))) {
							sendType, dataOut, status = client.otherModule(msg)
							break
						}
						//-354 -35401 -35402 -35403 -35404
						if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_GetBalanceRespond_N354))) {
							sendType, dataOut, status = client.otherModule(msg)
							break
						}
					}

					//-38
					if msg.Type == enum.MsgType_CloseChannelRequest_N38 ||
						msg.Type == enum.MsgType_CloseChannelSign_N39 {
						sendType, dataOut, status = client.channelModule(msg)
						break
					}

					//-40 -41
					if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_HTLC_RequestH_N40))) ||
						strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_HTLC_RespondH_N41))) {
						sendType, dataOut, status = client.htlcHDealModule(msg)
						break
					}
					//-42 -43
					if strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_HTLC_FindPathAndSendH_N42))) ||
						strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_HTLC_SignGetH_N43))) ||
						strings.HasPrefix(typeStr, strconv.Itoa(int(enum.MsgType_HTLC_SendH_N45))) {
						sendType, dataOut, status = client.htlcTxModule(msg)
						break
					}
					break
				}
			}
		}

		if len(dataOut) == 0 {
			dataOut = dataReq
		}

		//broadcast except me
		if sendType == enum.SendTargetType_SendToExceptMe {
			for itemClient := range GlobalWsClientManager.ClientsMap {
				if itemClient != itemClient {
					jsonMessage := getReplyObj(string(dataOut), msg.Type, status, client, itemClient)
					itemClient.SendChannel <- jsonMessage
				}
			}
		}
		//broadcast to all
		if sendType == enum.SendTargetType_SendToAll {
			jsonMessage := getReplyObj(string(dataOut), msg.Type, status, client, nil)
			GlobalWsClientManager.Broadcast <- jsonMessage
		}
	}
}

func getReplyObj(data string, msgType enum.MsgType, status bool, fromClient, toClient *Client) []byte {
	var jsonMessage []byte

	fromId := fromClient.Id
	if fromClient.User != nil {
		fromId = fromClient.User.PeerId
	}

	toClientId := "all"
	if toClient != nil {
		toClientId = toClient.Id
		if toClient.User != nil {
			toClientId = toClient.User.PeerId
		}
	}

	node := make(map[string]interface{})
	err := json.Unmarshal([]byte(data), &node)
	if err == nil {
		parse := gjson.Parse(data)
		jsonMessage, _ = json.Marshal(&bean.ReplyMessage{Type: msgType, Status: status, From: fromId, To: toClientId, Result: parse.Value()})
	} else {
		if strings.Contains(err.Error(), " array into Go value of type map") {
			parse := gjson.Parse(data)
			jsonMessage, _ = json.Marshal(&bean.ReplyMessage{Type: msgType, Status: status, From: fromId, To: toClientId, Result: parse.Value()})
		} else {
			jsonMessage, _ = json.Marshal(&bean.ReplyMessage{Type: msgType, Status: status, From: fromId, To: toClientId, Result: data})
		}
	}
	return jsonMessage
}

func (client *Client) sendToMyself(msgType enum.MsgType, status bool, data string) {
	jsonMessage := getReplyObj(data, msgType, status, client, client)
	client.SendChannel <- jsonMessage
}

func (client *Client) sendToSomeone(msgType enum.MsgType, status bool, recipientPeerId string, data string) error {
	if tool.CheckIsString(&recipientPeerId) {
		itemClient := GlobalWsClientManager.OnlineUserMap[recipientPeerId]
		if itemClient != nil && itemClient.User != nil {
			jsonMessage := getReplyObj(data, msgType, status, client, itemClient)
			itemClient.SendChannel <- jsonMessage
			return nil
		}
	}
	return errors.New("recipient not exist or online")
}
func (client *Client) FindUser(peerId *string) (*Client, error) {
	if tool.CheckIsString(peerId) {
		itemClient := GlobalWsClientManager.OnlineUserMap[*peerId]
		if itemClient != nil && itemClient.User != nil {
			return itemClient, nil
		}
	}
	return nil, errors.New("user not exist or online")
}

package lightclient

import (
	"encoding/json"
	"errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/gorilla/websocket"
	"github.com/omnilaboratory/obd/bean"
	"github.com/omnilaboratory/obd/bean/enum"
	"github.com/omnilaboratory/obd/config"
	"github.com/omnilaboratory/obd/dao"
	"github.com/omnilaboratory/obd/service"
	"github.com/omnilaboratory/obd/tool"
	trackerBean "github.com/omnilaboratory/obd/tracker/bean"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"
)

var conn *websocket.Conn
var interrupt chan os.Signal
var ticker3m *time.Ticker

func ConnectToTracker() (err error) {
	interrupt = make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: config.TrackerHost, Path: "/ws"}
	log.Printf("begin to connect to tracker: %s", u.String())

	conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("error ================ fail to dial tracker:", err)
		return err
	}
	service.TrackerWsConn = conn
	nodeId := httpCheckChainTypeByTracker()
	if nodeId == 0 {
		return errors.New("fail to login tracker")
	}
	if ticker3m != nil {
		startSchedule()
	}
	return nil
}

func SynData() {

	defer conn.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				conn = nil
				return
			}
			//log.Println("recv from tracker: " + string(message))

			replyMessage := bean.ReplyMessage{}
			err = json.Unmarshal(message, &replyMessage)
			if err == nil {
				switch replyMessage.Type {
				case enum.MsgType_Tracker_GetHtlcPath_351:
					v := reflect.ValueOf(replyMessage.Result)
					requestMessage := bean.RequestMessage{}
					requestMessage.Type = replyMessage.Type
					requestMessage.Data = ""
					if v.Kind() == reflect.Map {
						dataMap := replyMessage.Result.(map[string]interface{})
						requestMessage.RecipientUserPeerId = dataMap["senderPeerId"].(string)
						requestMessage.Data = dataMap["h"].(string) + "_" + dataMap["path"].(string)
					}
					htlcTrackerDealModule(requestMessage)
				}
			}
		}
	}()
	updateP2pAddressLogin()
	sycUserInfos()
	sycChannelInfos()

	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			info := make(map[string]interface{})
			info["type"] = enum.MsgType_Tracker_HeartBeat_302
			info["data"] = t.String()
			bytes, err := json.Marshal(info)
			err = conn.WriteMessage(websocket.TextMessage, bytes)
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("ws to tracker interrupt")
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				conn = nil
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func httpCheckChainTypeByTracker() (nodeId int) {
	bean.MyObdNodeInfo.TrackerNodeId = tool.GetObdNodeId()
	url := "http://" + config.TrackerHost + "/api/v1/checkChainType?nodeId=" + tool.GetObdNodeId() + "&chainType=" + config.ChainNode_Type
	resp, err := http.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return int(gjson.Get(string(body), "data").Get("id").Int())
	}
	return 0
}

func updateP2pAddressLogin() {
	info := make(map[string]interface{})
	info["type"] = enum.MsgType_Tracker_NodeLogin_303
	nodeLoginInfo := &trackerBean.ObdNodeLoginRequest{}
	nodeLoginInfo.NodeId = tool.GetObdNodeId()
	nodeLoginInfo.P2PAddress = localServerDest
	info["data"] = nodeLoginInfo
	bytes, err := json.Marshal(info)
	if err != nil {
		log.Println(err)
	} else {
		sendMsgToTracker(string(bytes))
	}
}

func sycUserInfos() {

	nodes := make([]trackerBean.ObdNodeUserLoginRequest, 0)
	for userId, _ := range globalWsClientManager.OnlineUserMap {
		user := trackerBean.ObdNodeUserLoginRequest{}
		user.UserId = userId
		nodes = append(nodes, user)
	}
	if len(nodes) > 0 {
		log.Println("syn channel data to tracker", nodes)
		info := make(map[string]interface{})
		info["type"] = enum.MsgType_Tracker_UpdateUserInfo_353
		info["data"] = nodes
		bytes, err := json.Marshal(info)
		if err == nil {
			sendMsgToTracker(string(bytes))
		}
	}
}

//同步通道信息
func sycChannelInfos() {
	_dir := "dbdata" + config.ChainNode_Type
	files, _ := ioutil.ReadDir(_dir)

	dbNames := make([]string, 0)
	userPeerIds := make([]string, 0)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "user_") && strings.HasSuffix(f.Name(), ".db") {
			peerId := strings.TrimPrefix(f.Name(), "user_")
			peerId = strings.TrimSuffix(peerId, ".db")
			value, exists := globalWsClientManager.OnlineUserMap[peerId]
			if exists && value != nil {
				userPeerIds = append(userPeerIds, peerId)
			} else {
				dbNames = append(dbNames, f.Name())
			}
		}
	}

	nodes := make([]trackerBean.ChannelInfoRequest, 0)

	for _, peerId := range userPeerIds {
		client, _ := globalWsClientManager.OnlineUserMap[peerId]
		if client != nil {
			checkChannel(client.User.Db, nodes)
		}
	}

	for _, dbName := range dbNames {
		db, err := storm.Open(_dir + "/" + dbName)
		if err == nil {
			checkChannel(db, nodes)
			_ = db.Close()
		}
	}

	for _, dbName := range dbNames {
		db, err := storm.Open(_dir + "/" + dbName)
		if err == nil {
			var channelInfos []dao.ChannelInfo
			err = db.All(&channelInfos)
			if err == nil {
				for _, channelInfo := range channelInfos {
					if len(channelInfo.ChannelId) > 0 {
						if channelInfo.CurrState == dao.ChannelState_CanUse || channelInfo.CurrState == dao.ChannelState_Close || channelInfo.CurrState == dao.ChannelState_HtlcTx {
							commitmentTransaction := dao.CommitmentTransaction{}
							err = db.Select(q.Eq("ChannelId", channelInfo.ChannelId)).OrderBy("CreateAt").Reverse().First(&commitmentTransaction)
							if err == nil {
								request := trackerBean.ChannelInfoRequest{}
								request.ChannelId = channelInfo.ChannelId
								request.PropertyId = channelInfo.PropertyId
								request.PeerIdA = channelInfo.PeerIdA
								request.PeerIdB = channelInfo.PeerIdB
								request.CurrState = channelInfo.CurrState
								request.AmountA = commitmentTransaction.AmountToRSMC
								request.AmountB = commitmentTransaction.AmountToCounterparty
								request.IsAlice = false
								if commitmentTransaction.Owner == channelInfo.PeerIdA {
									request.IsAlice = true
									request.AmountA = commitmentTransaction.AmountToRSMC
									request.AmountB = commitmentTransaction.AmountToCounterparty
								} else {
									request.AmountB = commitmentTransaction.AmountToRSMC
									request.AmountA = commitmentTransaction.AmountToCounterparty
								}
								nodes = append(nodes, request)
							}
						}
					}
				}
			}
			_ = db.Close()
		}
	}
	if len(nodes) > 0 {
		info := make(map[string]interface{})
		info["type"] = enum.MsgType_Tracker_UpdateChannelInfo_350
		info["data"] = nodes
		bytes, err := json.Marshal(info)
		if err == nil {
			sendMsgToTracker(string(bytes))
		}
	}
}

func checkChannel(db storm.Node, nodes []trackerBean.ChannelInfoRequest) {
	var channelInfos []dao.ChannelInfo
	err := db.All(&channelInfos)
	if err == nil {
		for _, channelInfo := range channelInfos {
			if len(channelInfo.ChannelId) > 0 {
				if channelInfo.CurrState == dao.ChannelState_CanUse || channelInfo.CurrState == dao.ChannelState_Close || channelInfo.CurrState == dao.ChannelState_HtlcTx {
					commitmentTransaction := dao.CommitmentTransaction{}
					err = db.Select(q.Eq("ChannelId", channelInfo.ChannelId)).OrderBy("CreateAt").Reverse().First(&commitmentTransaction)
					if err == nil {
						request := trackerBean.ChannelInfoRequest{}
						request.ChannelId = channelInfo.ChannelId
						request.PropertyId = channelInfo.PropertyId
						request.PeerIdA = channelInfo.PeerIdA
						request.PeerIdB = channelInfo.PeerIdB
						request.CurrState = channelInfo.CurrState
						request.AmountA = commitmentTransaction.AmountToRSMC
						request.AmountB = commitmentTransaction.AmountToCounterparty
						request.IsAlice = false
						if commitmentTransaction.Owner == channelInfo.PeerIdA {
							request.IsAlice = true
							request.AmountA = commitmentTransaction.AmountToRSMC
							request.AmountB = commitmentTransaction.AmountToCounterparty
						} else {
							request.AmountB = commitmentTransaction.AmountToRSMC
							request.AmountA = commitmentTransaction.AmountToCounterparty
						}
						nodes = append(nodes, request)
					}
				}
			}
		}
	}
}

func sendMsgToTracker(msg string) {
	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		log.Println("write:", err)
		return
	}
}

func startSchedule() {

	go func() {
		ticker3m = time.NewTicker(3 * time.Minute)
		defer ticker3m.Stop()
		for {
			select {
			case t := <-ticker3m.C:
				log.Println("timer 3min", t)
				if conn == nil {
					_ = ConnectToTracker()
				}
			}
		}
	}()
}

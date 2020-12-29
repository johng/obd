package lightclient

import (
	"encoding/json"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/gorilla/websocket"
	"github.com/omnilaboratory/obd/bean"
	"github.com/omnilaboratory/obd/bean/enum"
	"github.com/omnilaboratory/obd/config"
	"github.com/omnilaboratory/obd/conn"
	"github.com/omnilaboratory/obd/dao"
	"github.com/omnilaboratory/obd/service"
	"github.com/omnilaboratory/obd/tool"
	trackerBean "github.com/omnilaboratory/obd/tracker/bean"
	"io/ioutil"
	"log"
	"net/url"
	"reflect"
	"strings"
	"time"
)

var conn *websocket.Conn
var ticker3m *time.Ticker
var isReset = true

func ConnectToTracker() (err error) {

	u := url.URL{Scheme: "ws", Host: config.TrackerHost, Path: "/ws"}
	log.Printf("begin to connect to tracker: %s", u.String())

	conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("fail to dial tracker:", err)
		return err
	}

	chainNodeType, trackerP2pAddress, err := conn2tracker.GetChainNodeType()
	if err != nil {
		return err
	}
	config.ChainNodeType = chainNodeType
	config.BootstrapPeers, _ = config.StringsToAddrs(strings.Split(trackerP2pAddress, ","))

	err = StartP2PNode()
	if err != nil {
		return err
	}

	if service.TrackerChan == nil {
		service.TrackerChan = make(chan []byte)
	}

	if isReset {
		go readDataFromWs()
	}

	if ticker3m == nil {
		startSchedule()
	}
	return nil
}

func readDataFromWs() {
	isReset = false
	ticker := time.NewTicker(time.Minute * 2)
	defer ticker.Stop()

	defer func(ticker *time.Ticker) {
		if r := recover(); r != nil {
			log.Println("tracker goroutine recover")
			ticker.Stop()
			conn = nil
			isReset = true
		}
	}(ticker)
	defer conn.Close()

	// read message
	go func() {
		for {
			if conn == nil {
				isReset = true
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				isReset = true
				log.Println("socket to tracker get err:", err)
				conn = nil
				return
			}

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
						requestMessage.Data = dataMap["h"].(string) + "_" + dataMap["path"].(string) + "_" + tool.FloatToString(dataMap["amount"].(float64), 8)
						//requestMessage.Data = dataMap["h"].(string) + "_" + dataMap["path"].(string)
					}
					htlcTrackerDealModule(requestMessage)
				case enum.MsgType_Tracker_Connect_301:
					config.ChainNodeType = replyMessage.Result.(string)
					go SynData()
				}
			}
		}
	}()

	// heartbeat and check whether
	for {
		select {
		case t := <-ticker.C:
			if conn != nil {
				info := make(map[string]interface{})
				info["type"] = enum.MsgType_Tracker_HeartBeat_302
				info["data"] = t.String()
				bytes, err := json.Marshal(info)
				err = conn.WriteMessage(websocket.TextMessage, bytes)
				if err != nil {
					log.Println("HeartBeat:", err)
					isReset = true
					log.Println("socket to tracker get err:", err)
					conn = nil
					return
				}
			} else {
				return
			}
		}
	}
}

func SynData() {
	log.Println("synData to tracker")
	updateP2pAddressLogin()
	sycUserInfos()
	sycChannelInfos()
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
		sendMsgToTracker(bytes)
	}
}

func sycUserInfos() {

	nodes := make([]trackerBean.ObdNodeUserLoginRequest, 0)
	for userId, _ := range globalWsClientManager.OnlineClientMap {
		user := trackerBean.ObdNodeUserLoginRequest{}
		user.UserId = userId
		nodes = append(nodes, user)
	}
	if len(nodes) > 0 {
		log.Println("syn UserInfo data to tracker", nodes)
		info := make(map[string]interface{})
		info["type"] = enum.MsgType_Tracker_UpdateUserInfo_353
		info["data"] = nodes
		bytes, err := json.Marshal(&info)
		if err == nil {
			sendMsgToTracker(bytes)
		}
	}
}

//同步通道信息
func sycChannelInfos() {
	_dir := "dbdata" + config.ChainNodeType
	files, _ := ioutil.ReadDir(_dir)

	dbNames := make([]string, 0)
	userPeerIds := make([]string, 0)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "user_") && strings.HasSuffix(f.Name(), ".db") {
			peerId := strings.TrimPrefix(f.Name(), "user_")
			peerId = strings.TrimSuffix(peerId, ".db")
			value, exists := globalWsClientManager.OnlineClientMap[peerId]
			if exists && value != nil {
				userPeerIds = append(userPeerIds, peerId)
			} else {
				dbNames = append(dbNames, f.Name())
			}
		}
	}

	nodes := make([]trackerBean.ChannelInfoRequest, 0)

	for _, peerId := range userPeerIds {
		client, _ := globalWsClientManager.OnlineClientMap[peerId]
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
			err = db.Select(
				q.Eq("IsPrivate", false),
				q.Or(
					q.Eq("CurrState", dao.ChannelState_CanUse),
					q.Eq("CurrState", dao.ChannelState_Close),
					q.Eq("CurrState", dao.ChannelState_HtlcTx))).Find(&channelInfos)
			if err == nil {
				for _, channelInfo := range channelInfos {
					if len(channelInfo.ChannelId) > 0 && channelInfo.IsPrivate == false {
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
			sendMsgToTracker(bytes)
		}
	}
}

func checkChannel(db storm.Node, nodes []trackerBean.ChannelInfoRequest) {
	var channelInfos []dao.ChannelInfo
	err := db.Select(
		q.Eq("IsPrivate", false),
		q.Or(
			q.Eq("CurrState", dao.ChannelState_CanUse),
			q.Eq("CurrState", dao.ChannelState_Close),
			q.Eq("CurrState", dao.ChannelState_HtlcTx))).Find(&channelInfos)
	if err == nil {
		for _, channelInfo := range channelInfos {
			if len(channelInfo.ChannelId) > 0 && channelInfo.IsPrivate == false {
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

func sendMsgToTracker(msg []byte) {
	//log.Println("send to tracker", string(msg))
	if conn == nil {
		isReset = true
		err := ConnectToTracker()
		if err != nil {
			log.Println(err)
			return
		}
	}
	err := conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Println("write:", err)
		return
	}
}

func startSchedule() {
	go func() {
		for {
			select {
			case msg := <-service.TrackerChan:
				sendMsgToTracker(msg)
			}
		}
	}()

	go func() {
		ticker3m = time.NewTicker(1 * time.Minute)
		defer ticker3m.Stop()
		for {
			select {
			case t := <-ticker3m.C:
				if conn == nil {
					log.Println("reconnect tracker ", t)
					_ = ConnectToTracker()
				}
			}
		}
	}()
}

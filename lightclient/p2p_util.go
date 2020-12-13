package lightclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/omnilaboratory/obd/bean"
	"github.com/omnilaboratory/obd/config"
	"github.com/omnilaboratory/obd/service"
	"github.com/omnilaboratory/obd/tool"
	"log"
	"math/rand"
	"strings"
	"time"
)

type P2PChannel struct {
	IsLocalChannel bool
	Address        string
	stream         network.Stream
	rw             *bufio.ReadWriter
}

const pid = "/chat/1.0.0"

var localServerDest string
var p2PLocalPeerId string
var privateKey crypto.PrivKey
var p2pChannelMap map[string]*P2PChannel

func generatePrivateKey() (crypto.PrivKey, error) {
	if privateKey == nil {
		r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
		prvKey, _, err := crypto.GenerateECDSAKeyPair(r)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		privateKey = prvKey
	}
	return privateKey, nil
}

func StartP2PServer() (err error) {
	prvKey, err := generatePrivateKey()
	if err != nil {
		return err
	}
	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.P2P_sourcePort))

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)
	if err != nil {
		log.Println(err)
		return err
	}
	p2pChannelMap = make(map[string]*P2PChannel)
	p2PLocalPeerId = host.ID().Pretty()
	service.P2PLocalPeerId = p2PLocalPeerId

	localServerDest = fmt.Sprintf("/ip4/%s/tcp/%v/p2p/%s", config.P2P_hostIp, config.P2P_sourcePort, host.ID().Pretty())
	bean.CurrObdNodeInfo.P2pAddress = localServerDest

	//把自己也作为终点放进去，阻止自己连接自己
	p2pChannelMap[p2PLocalPeerId] = &P2PChannel{
		IsLocalChannel: true,
		Address:        localServerDest,
	}
	host.SetStreamHandler(pid, handleStream)
	return nil
}

func connP2PServer(dest string) (string, error) {
	if tool.CheckIsString(&dest) == false {
		log.Println("wrong dest address")
		return "", errors.New("wrong dest address")
	}

	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.P2P_sourcePort))
	prvKey, _ := generatePrivateKey()

	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)

	if err != nil {
		log.Println(err)
		return "", err
	}
	destMaddr, err := multiaddr.NewMultiaddr(dest)
	if err != nil {
		log.Println(err)
		return "", err
	}

	destHostInfo, err := peer.AddrInfoFromP2pAddr(destMaddr)
	if err != nil {
		log.Println(err)
		return "", err
	}
	if p2pChannelMap[destHostInfo.ID.Pretty()] != nil {
		log.Println("Remote peer has been connected")
		return " Remote peer has been connected", nil
	}
	host.Peerstore().AddAddrs(destHostInfo.ID, destHostInfo.Addrs, peerstore.PermanentAddrTTL)
	s, err := host.NewStream(context.Background(), destHostInfo.ID, pid)
	if err != nil {
		log.Println(err)
		return "", err
	}
	rw := addP2PChannel(s)
	go readData(s, rw)
	return localServerDest, nil
}

func handleStream(s network.Stream) {
	if s != nil {
		if p2pChannelMap[s.Conn().RemotePeer().Pretty()] == nil {
			log.Println("Got a new stream!")
			rw := addP2PChannel(s)
			go readData(s, rw)
		}
	}
}
func readData(s network.Stream, rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('~')
		if err != nil {
			delete(p2pChannelMap, s.Conn().RemotePeer().Pretty())
			return
		}
		if str == "" {
			return
		}
		if str != "" {
			log.Println(s.Conn())
			str = strings.TrimSuffix(str, "~")
			reqData := &bean.RequestMessage{}
			err := json.Unmarshal([]byte(str), reqData)
			if err == nil {
				err = getDataFromP2PSomeone(*reqData)
				if err != nil {
					//msg := err.Error() + "~"
					//_, _ = rw.WriteString(msg)
					//_ = rw.Flush()
				}
			}
		}
	}
}

func addP2PChannel(stream network.Stream) *bufio.ReadWriter {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	node := &P2PChannel{
		Address: stream.Conn().RemoteMultiaddr().String() + "/" + stream.Conn().RemotePeer().String(),
		stream:  stream,
		rw:      rw,
	}
	p2pChannelMap[stream.Conn().RemotePeer().Pretty()] = node
	return rw
}

func sendP2PMsg(remoteP2PPeerId string, msg string) error {
	if tool.CheckIsString(&remoteP2PPeerId) == false {
		return errors.New("empty remoteP2PPeerId")
	}
	if remoteP2PPeerId == p2PLocalPeerId {
		return errors.New("remoteP2PPeerId is yourself,can not send msg to yourself")
	}

	channel := p2pChannelMap[remoteP2PPeerId]
	if channel != nil {
		msg = msg + "~"
		_, _ = channel.rw.WriteString(msg)
		_ = channel.rw.Flush()
	}
	return nil
}

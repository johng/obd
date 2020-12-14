package cfg

import (
	"flag"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-ini/ini"
)

var (
	//Cfg               *ini.File
	configPath        = flag.String("trackerConfigPath", "tracker/config/conf.ini", "Config file path")
	ReadTimeout       = 60 * time.Second
	WriteTimeout      = 60 * time.Second
	TrackerServerPort = 60060

	HtlcFeeRate = 0.0001

	ChainNode_Type = "test"
	ChainNode_Host = "62.234.216.108:18332"
	ChainNode_User = "omniwallet"
	ChainNode_Pass = "cB3]iL2@eZ1?cB2?"
)

func parseHostname(hostname string) string {
	P2pHostIps, err := net.LookupIP(hostname)
	if err != nil {
		panic("Can't parse hostname")
	}

	return P2pHostIps[0].String()
}

func init() {
	testing.Init()
	flag.Parse()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	Cfg, err := ini.Load(*configPath)
	if err != nil {
		if strings.Contains(err.Error(), "open tracker/config/conf.ini") {
			Cfg, err = ini.Load("config/conf.ini")
			if err != nil {
				log.Println(err)
				return
			}
		}
	}

	section, err := Cfg.GetSection("server")
	if err != nil {
		log.Println(err)
		return
	}
	TrackerServerPort = section.Key("port").MustInt(60060)

	htlcNode, err := Cfg.GetSection("htlc")
	if err != nil {
		log.Println(err)
		return
	}
	HtlcFeeRate = htlcNode.Key("feeRate").MustFloat64(0.0001)

	chainNode, err := Cfg.GetSection("chainNode")
	if err != nil {
		log.Println(err)
		return
	}

	RawHostIP := strings.Split(chainNode.Key("host").String(), ":")
	ParseHostname := parseHostname(RawHostIP[0])
	ChainNode_Host = ParseHostname + ":" + RawHostIP[1]
	ChainNode_User = chainNode.Key("user").String()
	ChainNode_Pass = chainNode.Key("pass").String()
	if len(ChainNode_Host) == 0 {
		log.Println("empty omnicore host")
		return
	}
	if len(ChainNode_User) == 0 {
		log.Println("empty omnicore account")
		return
	}
	if len(ChainNode_Pass) == 0 {
		log.Println("empty omnicore password")
		return
	}
}

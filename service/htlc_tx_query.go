package service

import (
	"errors"
	"github.com/asdine/storm/q"
	"github.com/omnilaboratory/obd/bean"
	"github.com/omnilaboratory/obd/bean/enum"
	"github.com/omnilaboratory/obd/dao"
	"github.com/omnilaboratory/obd/tool"
	"github.com/tidwall/gjson"
	"sync"
)

type htlcQueryTxManager struct {
	operationFlag sync.Mutex
}

var HtlcQueryTxManager htlcQueryTxManager

func (service *htlcQueryTxManager) GetLatestHT1aOrHE1b(msgData string, user bean.User) (data interface{}, err error) {
	if tool.CheckIsString(&msgData) == false {
		return nil, errors.New(enum.Tips_common_empty + "msg data")
	}
	channelId := gjson.Get(msgData, "channel_id").Str
	if tool.CheckIsString(&channelId) == false {
		return nil, errors.New(enum.Tips_common_empty + "channel_id")
	}
	tx, _ := user.Db.Begin(true)
	defer tx.Rollback()

	commitmentTxInfo, err := getLatestCommitmentTxUseDbTx(tx, channelId, user.PeerId)
	if err != nil {
		return nil, errors.New(enum.Tips_channel_notFoundLatestCommitmentTx)
	}
	ht1aOrHe1b := dao.HTLCTimeoutTxForAAndExecutionForB{}
	_ = tx.Select(q.Eq("ChannelId", channelId), q.Eq("CommitmentTxId", commitmentTxInfo.Id)).First(&ht1aOrHe1b)
	if ht1aOrHe1b.Id == 0 {
		return nil, errors.New(enum.Tips_common_notFound)
	}
	_ = tx.Commit()
	return ht1aOrHe1b, nil
}

func (service *htlcQueryTxManager) GetHT1aOrHE1bBySomeCommitmentId(msgData string, user bean.User) (data interface{}, err error) {
	if tool.CheckIsString(&msgData) == false {
		return nil, errors.New(enum.Tips_common_empty + "msg data")
	}

	channelId := gjson.Get(msgData, "channel_id").Str
	if tool.CheckIsString(&channelId) == false {
		return nil, errors.New(enum.Tips_common_empty + "channel_id")
	}
	commitmentTxId := gjson.Get(msgData, "commitment_tx_id").Int()
	if commitmentTxId < 1 {
		return nil, errors.New(enum.Tips_common_wrong + "commitment_tx_id")
	}
	tx, _ := user.Db.Begin(true)
	defer tx.Rollback()
	commitmentTransaction := dao.CommitmentTransaction{}
	_ = tx.Select(q.Eq("ChannelId", channelId), q.Eq("Id", commitmentTxId)).First(&commitmentTransaction)
	if commitmentTransaction.Id == 0 {
		return nil, errors.New("commitmentTransaction " + enum.Tips_common_notFound)
	}

	ht1aOrHe1b := dao.HTLCTimeoutTxForAAndExecutionForB{}
	_ = tx.Select(q.Eq("ChannelId", channelId), q.Eq("CommitmentTxId", commitmentTransaction.Id)).First(&ht1aOrHe1b)
	if ht1aOrHe1b.Id == 0 {
		return nil, errors.New(enum.Tips_common_notFound)
	}
	_ = tx.Commit()
	return ht1aOrHe1b, nil
}

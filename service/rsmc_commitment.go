package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/omnilaboratory/obd/bean"
	"github.com/omnilaboratory/obd/bean/enum"
	"github.com/omnilaboratory/obd/dao"
	"github.com/omnilaboratory/obd/rpc"
	"github.com/omnilaboratory/obd/tool"
	"github.com/tidwall/gjson"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
)

type commitmentTxManager struct {
	operationFlag sync.Mutex
}

var CommitmentTxService commitmentTxManager

func (this *commitmentTxManager) CommitmentTransactionCreated(msg bean.RequestMessage, creator *bean.User) (retData *bean.PayerRequestCommitmentTxOfP2p, err error) {
	if tool.CheckIsString(&msg.Data) == false {
		return nil, errors.New(enum.Tips_common_empty + "msg.Data")
	}

	reqData := &bean.SendRequestCommitmentTx{}
	err = json.Unmarshal([]byte(msg.Data), reqData)
	if err != nil {
		return nil, err
	}
	if tool.CheckIsString(&reqData.ChannelId) == false {
		return nil, errors.New(enum.Tips_common_empty + " channel_id")
	}

	if tool.CheckIsString(&reqData.LastTempAddressPrivateKey) == false {
		return nil, errors.New(enum.Tips_common_empty + "last_temp_address_private_key")
	}

	if tool.CheckIsString(&reqData.ChannelAddressPrivateKey) == false {
		return nil, errors.New(enum.Tips_common_empty + "channel_address_private_key")
	}

	if reqData.Amount <= 0 {
		return nil, errors.New(enum.Tips_common_wrong + "payment amount")
	}

	tx, err := creator.Db.Begin(true)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer tx.Rollback()

	channelInfo := getChannelInfoByChannelId(tx, reqData.ChannelId, creator.PeerId)
	if channelInfo == nil {
		err = errors.New(enum.Tips_funding_notFoundChannelByChannelId + reqData.ChannelId)
		log.Println(err)
		return nil, err
	}

	if channelInfo.CurrState == dao.ChannelState_NewTx {
		return nil, errors.New(enum.Tips_common_newTxMsg)
	}

	fundingTransaction := getFundingTransactionByChannelId(tx, channelInfo.ChannelId, creator.PeerId)
	duration := time.Now().Sub(fundingTransaction.CreateAt)
	if duration > time.Minute*30 {
		if checkChannelOmniAssetAmount(*channelInfo) == false {
			err = errors.New(enum.Tips_rsmc_broadcastedChannel)
			log.Println(err)
			return nil, err
		}
	}

	err = checkBtcFundFinish(channelInfo.ChannelAddress, false)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	senderPubKey := channelInfo.PubKeyA
	targetUser := channelInfo.PeerIdB
	if creator.PeerId == channelInfo.PeerIdB {
		senderPubKey = channelInfo.PubKeyB
		targetUser = channelInfo.PeerIdA
	}

	if targetUser != msg.RecipientUserPeerId {
		return nil, errors.New(enum.Tips_rsmc_notTargetUser + msg.RecipientUserPeerId)
	}

	latestCommitmentTxInfo, err := getLatestCommitmentTxUseDbTx(tx, reqData.ChannelId, creator.PeerId)
	if err != nil {
		return nil, errors.New(enum.Tips_channel_notFoundLatestCommitmentTx)
	}

	if latestCommitmentTxInfo.TxType != dao.CommitmentTransactionType_Rsmc {
		return nil, errors.New(enum.Tips_rsmc_errorCommitmentTxType + strconv.Itoa(int(latestCommitmentTxInfo.TxType)))
	}

	if latestCommitmentTxInfo.CurrState != dao.TxInfoState_CreateAndSign &&
		latestCommitmentTxInfo.CurrState != dao.TxInfoState_Create {
		return nil, errors.New(enum.Tips_rsmc_errorCommitmentTxState + strconv.Itoa(int(latestCommitmentTxInfo.CurrState)))
	}

	//region check input data 检测输入数据
	//如果是第一次发起请求
	if latestCommitmentTxInfo.CurrState == dao.TxInfoState_CreateAndSign {
		balance := latestCommitmentTxInfo.AmountToRSMC
		if balance < reqData.Amount {
			return nil, errors.New(enum.Tips_rsmc_notEnoughBalance)
		}

		if _, err = tool.GetPubKeyFromWifAndCheck(reqData.LastTempAddressPrivateKey, latestCommitmentTxInfo.RSMCTempAddressPubKey); err != nil {
			return nil, errors.New(fmt.Sprintf(enum.Tips_rsmc_wrongPrivateKeyForLast, reqData.LastTempAddressPrivateKey, latestCommitmentTxInfo.RSMCTempAddressPubKey))
		}
	} else {
		if reqData.CurrTempAddressPubKey != latestCommitmentTxInfo.RSMCTempAddressPubKey {
			return nil, errors.New(fmt.Sprintf(enum.Tips_rsmc_notSameValueWhenCreate, reqData.CurrTempAddressPubKey, latestCommitmentTxInfo.RSMCTempAddressPubKey))
		}
		lastCommitmentTx := &dao.CommitmentTransaction{}
		_ = tx.One("Id", latestCommitmentTxInfo.LastCommitmentTxId, lastCommitmentTx)
		if _, err = tool.GetPubKeyFromWifAndCheck(reqData.LastTempAddressPrivateKey, lastCommitmentTx.RSMCTempAddressPubKey); err != nil {
			return nil, err
		}
	}

	if _, err = tool.GetPubKeyFromWifAndCheck(reqData.ChannelAddressPrivateKey, senderPubKey); err != nil {
		return nil, err
	}
	tempAddrPrivateKeyMap[senderPubKey] = reqData.ChannelAddressPrivateKey

	if _, err = getAddressFromPubKey(reqData.CurrTempAddressPubKey); err != nil {
		return nil, errors.New(enum.Tips_common_wrong + "curr_temp_address_pub_key")
	}
	if tool.CheckIsString(&reqData.CurrTempAddressPrivateKey) == false {
		return nil, errors.New(enum.Tips_common_empty + "curr_temp_address_private_key")
	}
	if _, err = tool.GetPubKeyFromWifAndCheck(reqData.CurrTempAddressPrivateKey, reqData.CurrTempAddressPubKey); err != nil {
		return nil, errors.New(fmt.Sprintf(enum.Tips_rsmc_notPairPrivAndPubKey, reqData.CurrTempAddressPrivateKey, reqData.CurrTempAddressPubKey))
	}
	tempAddrPrivateKeyMap[reqData.CurrTempAddressPubKey] = reqData.CurrTempAddressPrivateKey
	//endregion

	retData = &bean.PayerRequestCommitmentTxOfP2p{}
	retData.ChannelId = channelInfo.ChannelId
	retData.Amount = reqData.Amount
	retData.LastTempAddressPrivateKey = reqData.LastTempAddressPrivateKey
	retData.CurrTempAddressPubKey = reqData.CurrTempAddressPubKey
	if latestCommitmentTxInfo.CurrState == dao.TxInfoState_CreateAndSign {
		//创建c2a omni的交易不能一个输入，多个输出，所以就是两个交易
		newCommitmentTxInfo, err := createCommitmentTxHex(tx, true, reqData, channelInfo, latestCommitmentTxInfo, *creator)
		if err != nil {
			return nil, err
		}
		retData.RsmcHex = newCommitmentTxInfo.RSMCTxHex
		retData.ToCounterpartyTxHex = newCommitmentTxInfo.ToCounterpartyTxHex
		retData.CommitmentTxHash = newCommitmentTxInfo.CurrHash
	} else {
		retData.RsmcHex = latestCommitmentTxInfo.RSMCTxHex
		retData.ToCounterpartyTxHex = latestCommitmentTxInfo.ToCounterpartyTxHex
		retData.CommitmentTxHash = latestCommitmentTxInfo.CurrHash
	}

	retData.PayerNodeAddress = msg.SenderNodePeerId
	retData.PayerPeerId = msg.SenderUserPeerId
	_ = tx.Commit()

	return retData, err
}

//353 352的请求阶段完成，需要Alice这边签名C2b等相关的交易
func (this *commitmentTxManager) AfterBobSignCommitmentTransactionAtAliceSide(data string, user *bean.User) (retData map[string]interface{}, needNoticeAlice bool, err error) {
	signCommitmentTx := &bean.PayeeSignCommitmentTxOfP2p{}
	_ = json.Unmarshal([]byte(data), signCommitmentTx)

	//region 检测传入数据
	var channelId = signCommitmentTx.ChannelId
	if tool.CheckIsString(&channelId) == false {
		err = errors.New("wrong channelId")
		log.Println(err)
		return nil, false, err
	}
	var commitmentTxHash = signCommitmentTx.CommitmentTxHash
	if tool.CheckIsString(&commitmentTxHash) == false {
		err = errors.New("wrong commitmentTxHash")
		log.Println(err)
		return nil, false, err
	}

	tx, err := user.Db.Begin(true)
	if err != nil {
		log.Println(err)
		return nil, true, err
	}
	defer tx.Rollback()

	channelInfo := getChannelInfoByChannelId(tx, channelId, user.PeerId)
	if channelInfo == nil {
		return nil, true, errors.New("not found channelInfo at targetSide")
	}

	latestCommitmentTxInfo, err := getLatestCommitmentTxUseDbTx(tx, channelId, user.PeerId)
	if err != nil {
		err = errors.New("fail to find sender's commitmentTxInfo")
		log.Println(err)
		return nil, true, err
	}

	if latestCommitmentTxInfo.CurrHash != commitmentTxHash {
		err = errors.New("wrong request hash, Please notice payee,")
		log.Println(err)
		return nil, true, err
	}

	if latestCommitmentTxInfo.CurrState != dao.TxInfoState_Create {
		err = errors.New("wrong commitmentTxInfo state " + strconv.Itoa(int(latestCommitmentTxInfo.CurrState)))
		log.Println(err)
		return nil, false, err
	}

	aliceData := make(map[string]interface{})
	aliceData["approval"] = signCommitmentTx.Approval
	aliceData["channel_id"] = signCommitmentTx.ChannelId

	retData = make(map[string]interface{})
	retData["aliceData"] = aliceData
	if signCommitmentTx.Approval == false {
		_ = tx.DeleteStruct(latestCommitmentTxInfo)
		_ = tx.Commit()
		return retData, false, nil
	}

	var signedRsmcHex = signCommitmentTx.SignedRsmcHex
	var payerRsmcTestRetData string
	if tool.CheckIsString(&signedRsmcHex) {
		payerRsmcTestRetData, err = rpcClient.TestMemPoolAccept(signedRsmcHex)
		if err != nil {
			err = errors.New("wrong signedRsmcHex")
			log.Println(err)
			return nil, false, err
		}
	}

	var signedToOtherHex = signCommitmentTx.SignedToCounterpartyTxHex
	if tool.CheckIsString(&signedToOtherHex) == false {
		err = errors.New("wrong signedToOtherHex")
		log.Println(err)
		return nil, false, err
	}
	toCounterpartyTxid, err := rpcClient.TestMemPoolAccept(signedToOtherHex)
	if err != nil {
		err = errors.New("wrong signedToOtherHex")
		log.Println(err)
		return nil, false, err
	}

	var aliceRdHex = signCommitmentTx.PayerRdHex
	if tool.CheckIsString(&aliceRdHex) {
		_, err = rpcClient.TestMemPoolAccept(aliceRdHex)
		if err != nil {
			err = errors.New("wrong senderRdHex")
			log.Println(err)
			return nil, false, err
		}
	}

	var bobRsmcHex = signCommitmentTx.RsmcHex
	if tool.CheckIsString(&bobRsmcHex) == false {
		err = errors.New(enum.Tips_common_empty + "payee rsmcHex")
		log.Println(err)
		return nil, false, err
	}
	_, err = rpcClient.TestMemPoolAccept(bobRsmcHex)
	if err != nil {
		err = errors.New(enum.Tips_common_wrong + "payee rsmcHex")
		log.Println(err)
		return nil, false, err
	}

	var bobCurrTempAddressPubKey = signCommitmentTx.CurrTempAddressPubKey
	if tool.CheckIsString(&bobCurrTempAddressPubKey) == false {
		err = errors.New("wrong currTempAddressPubKey")
		log.Println(err)
		return nil, false, err
	}
	var bobToCounterpartyTxHex = signCommitmentTx.ToCounterpartyTxHex
	var bobLastTempAddressPrivateKey = signCommitmentTx.LastTempAddressPrivateKey
	//endregion

	bobData := make(map[string]interface{})

	fundingTransaction := getFundingTransactionByChannelId(tx, channelId, user.PeerId)
	if fundingTransaction == nil {
		return nil, true, errors.New("not found fundingTransaction at targetSide")
	}

	var myChannelPubKey = channelInfo.PubKeyA
	var myChannelAddress = channelInfo.AddressA
	var partnerChannelAddress = channelInfo.AddressB
	if user.PeerId == channelInfo.PeerIdB {
		myChannelAddress = channelInfo.AddressB
		myChannelPubKey = channelInfo.PubKeyB
		partnerChannelAddress = channelInfo.AddressA
	}
	var myChannelPrivateKey = tempAddrPrivateKeyMap[myChannelPubKey]

	//region 根据对方传过来的上一个交易的临时rsmc私钥，签名上一次的BR交易，保证对方确实放弃了上一个承诺交易
	err = signLastBR(tx, dao.BRType_Rmsc, *channelInfo, user.PeerId, bobLastTempAddressPrivateKey, latestCommitmentTxInfo.LastCommitmentTxId)
	if err != nil {
		log.Println(err)
		return nil, false, err
	}
	//endregion

	if tool.CheckIsString(&signedRsmcHex) {
		latestCommitmentTxInfo.RSMCTxHex = signedRsmcHex
		latestCommitmentTxInfo.RSMCTxid = gjson.Parse(payerRsmcTestRetData).Array()[0].Get("txid").Str
		// region 对自己的RD 二次签名
		err = signRdTx(tx, channelInfo, signedRsmcHex, aliceRdHex, latestCommitmentTxInfo, myChannelAddress, user)
		if err != nil {
			return nil, true, err
		}
		// endregion
	}

	//更新alice的当前承诺交易
	latestCommitmentTxInfo.CurrState = dao.TxInfoState_CreateAndSign
	latestCommitmentTxInfo.ToCounterpartyTxHex = signedToOtherHex
	latestCommitmentTxInfo.ToCounterpartyTxid = gjson.Parse(toCounterpartyTxid).Array()[0].Get("txid").Str
	latestCommitmentTxInfo.SignAt = time.Now()

	bytes, err := json.Marshal(latestCommitmentTxInfo)
	msgHash := tool.SignMsgWithSha256(bytes)
	latestCommitmentTxInfo.CurrHash = msgHash
	_ = tx.Update(latestCommitmentTxInfo)

	lastCommitmentTxInfo := dao.CommitmentTransaction{}
	err = tx.One("Id", latestCommitmentTxInfo.LastCommitmentTxId, &lastCommitmentTxInfo)
	if err == nil {
		lastCommitmentTxInfo.CurrState = dao.TxInfoState_Abord
		_ = tx.Update(lastCommitmentTxInfo)
	}

	aliceData["latest_commitment_tx_info"] = latestCommitmentTxInfo
	//处理对方的数据
	//签名对方传过来的rsmcHex
	bobRsmcTxid, bobSignedRsmcHex, err := rpcClient.BtcSignRawTransaction(bobRsmcHex, myChannelPrivateKey)
	if err != nil {
		return nil, false, errors.New("fail to sign rsmc hex ")
	}
	testResult, err := rpcClient.TestMemPoolAccept(bobSignedRsmcHex)
	if err != nil {
		return nil, false, err
	}
	if gjson.Parse(testResult).Array()[0].Get("allowed").Bool() == false {
		return nil, false, errors.New(gjson.Parse(testResult).Array()[0].Get("reject-reason").String())
	}
	err = checkBobRemcData(bobSignedRsmcHex, latestCommitmentTxInfo)
	if err != nil {
		return nil, false, err
	}
	bobData["signedRsmcHex"] = bobSignedRsmcHex

	//region create RD tx for bob
	bobMultiAddr, err := rpcClient.CreateMultiSig(2, []string{bobCurrTempAddressPubKey, myChannelPubKey})
	if err != nil {
		return nil, false, err
	}
	bobRsmcMultiAddress := gjson.Get(bobMultiAddr, "address").String()
	bobRsmcRedeemScript := gjson.Get(bobMultiAddr, "redeemScript").String()
	addressJson, err := rpcClient.GetAddressInfo(bobRsmcMultiAddress)
	if err != nil {
		return nil, false, err
	}
	bobRsmcMultiAddressScriptPubKey := gjson.Get(addressJson, "scriptPubKey").String()

	inputs, err := getInputsForNextTxByParseTxHashVout(
		bobSignedRsmcHex,
		bobRsmcMultiAddress,
		bobRsmcMultiAddressScriptPubKey,
		bobRsmcRedeemScript)
	if err != nil {
		log.Println(err)
		return nil, false, err
	}

	_, bobRdhex, err := rpcClient.OmniCreateAndSignRawTransactionUseUnsendInput(
		bobRsmcMultiAddress,
		[]string{
			myChannelPrivateKey,
		},
		inputs,
		partnerChannelAddress,
		channelInfo.FundingAddress,
		channelInfo.PropertyId,
		latestCommitmentTxInfo.AmountToCounterparty,
		getBtcMinerAmount(channelInfo.BtcAmount),
		1000,
		&bobRsmcRedeemScript)
	if err != nil {
		log.Println(err)
		return nil, false, errors.New("fail to create rd")
	}
	bobData["rdHex"] = bobRdhex
	//endregion create RD tx for alice

	//region 根据对对方的Rsmc签名，生成惩罚对方，自己获益BR
	bobCommitmentTx := &dao.CommitmentTransaction{}
	bobCommitmentTx.Id = latestCommitmentTxInfo.Id
	bobCommitmentTx.PropertyId = channelInfo.PropertyId
	bobCommitmentTx.RSMCTempAddressPubKey = bobCurrTempAddressPubKey
	bobCommitmentTx.RSMCMultiAddress = bobRsmcMultiAddress
	bobCommitmentTx.RSMCRedeemScript = bobRsmcRedeemScript
	bobCommitmentTx.RSMCMultiAddressScriptPubKey = bobRsmcMultiAddressScriptPubKey
	bobCommitmentTx.RSMCTxHex = bobSignedRsmcHex
	bobCommitmentTx.RSMCTxid = bobRsmcTxid
	bobCommitmentTx.AmountToRSMC = latestCommitmentTxInfo.AmountToCounterparty
	err = createCurrCommitmentTxBR(tx, dao.BRType_Rmsc, channelInfo, bobCommitmentTx, inputs, myChannelAddress, myChannelPrivateKey, *user)
	if err != nil {
		log.Println(err)
		return nil, false, err
	}
	//endregion

	//签名对方传过来的toOtherHex
	var bobSignedToCounterpartyTxHex string
	if tool.CheckIsString(&bobToCounterpartyTxHex) {
		_, bobSignedToCounterpartyTxHex, err = rpcClient.BtcSignRawTransaction(bobToCounterpartyTxHex, myChannelPrivateKey)
		if err != nil {
			return nil, false, errors.New("fail to sign to_counterparty_tx_hex")
		}
		testResult, err = rpcClient.TestMemPoolAccept(bobSignedToCounterpartyTxHex)
		if err != nil {
			return nil, false, err
		}
		if gjson.Parse(testResult).Array()[0].Get("allowed").Bool() == false {
			return nil, false, errors.New(gjson.Parse(testResult).Array()[0].Get("reject-reason").String())
		}
	}
	bobData["signedToCounterpartyTxHex"] = bobSignedToCounterpartyTxHex
	bobData["payerCommitmentTxHash"] = commitmentTxHash

	_ = tx.Commit()

	//同步通道信息到tracker
	sendChannelStateToTracker(*channelInfo, *latestCommitmentTxInfo)
	bobData["channelId"] = channelId
	retData["bobData"] = bobData
	return retData, true, nil
}

func (this *commitmentTxManager) SendSomeCommitmentById(data string, user *bean.User) (retData interface{}, err error) {
	id, err := strconv.Atoi(data)
	if err != nil {
		return nil, err
	}
	tx, err := user.Db.Begin(true)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer tx.Rollback()

	commitmentTransaction := &dao.CommitmentTransaction{}
	err = tx.One("Id", id, commitmentTransaction)
	if err != nil || commitmentTransaction.Id == 0 {
		return nil, err
	}
	if commitmentTransaction.CurrState != dao.TxInfoState_CreateAndSign {
		return nil, errors.New("wrong commitment state")
	}

	//region 广播承诺交易 最近的rsmc的资产分配交易 因为是omni资产，承诺交易被拆分成了两个独立的交易
	if tool.CheckIsString(&commitmentTransaction.RSMCTxHex) {
		commitmentTxid, err := rpcClient.SendRawTransaction(commitmentTransaction.RSMCTxHex)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		log.Println(commitmentTxid)
	}
	if tool.CheckIsString(&commitmentTransaction.ToCounterpartyTxHex) {
		commitmentTxidToBob, err := rpcClient.SendRawTransaction(commitmentTransaction.ToCounterpartyTxHex)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		log.Println(commitmentTxidToBob)
	}
	//endregion

	//region 广播RD
	latestRevocableDeliveryTx := &dao.RevocableDeliveryTransaction{}
	err = tx.Select(
		q.Eq("ChannelId", commitmentTransaction.ChannelId),
		q.Eq("CommitmentTxId", commitmentTransaction.Id),
		q.Eq("Owner", user.PeerId)).
		OrderBy("CreateAt").Reverse().
		First(latestRevocableDeliveryTx)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = rpcClient.SendRawTransaction(latestRevocableDeliveryTx.TxHex)
	if err != nil {
		log.Println(err)
		msg := err.Error()
		//如果omnicore返回的信息里面包含了non-BIP68-final (code 64)， 则说明因为需要等待1000个区块高度，广播是对的
		if strings.Contains(msg, "non-BIP68-final (code 64)") == false &&
			strings.Contains(msg, "Code: -25,Msg: Missing inputs") == false {
			return nil, err
		}
	}
	//endregion

	// region update state
	commitmentTransaction.CurrState = dao.TxInfoState_SendHex
	commitmentTransaction.SendAt = time.Now()
	err = tx.Update(commitmentTransaction)
	if err != nil {
		return nil, err
	}

	latestRevocableDeliveryTx.CurrState = dao.TxInfoState_SendHex
	latestRevocableDeliveryTx.SendAt = time.Now()
	err = tx.Update(latestRevocableDeliveryTx)
	if err != nil {
		return nil, err
	}

	err = addRDTxToWaitDB(latestRevocableDeliveryTx)
	if err != nil {
		return nil, err
	}
	//endregion

	return nil, nil
}

func checkBobRemcData(rsmcHex string, commitmentTransaction *dao.CommitmentTransaction) error {
	result, err := rpcClient.OmniDecodeTransaction(rsmcHex)
	if err != nil {
		return errors.New("error rsmcHex")
	}
	parse := gjson.Parse(result)
	if parse.Get("propertyid").Int() != commitmentTransaction.PropertyId {
		return errors.New("error propertyId in rsmcHex ")
	}
	if parse.Get("amount").Float() != commitmentTransaction.AmountToCounterparty {
		return errors.New("error amount in rsmcHex ")
	}
	return nil
}

type commitmentTxSignedManager struct {
	operationFlag sync.Mutex
}

var CommitmentTxSignedService commitmentTxSignedManager

func (this *commitmentTxSignedManager) BeforeBobSignCommitmentTransactionAtBobSide(data string, user *bean.User) (retData *bean.PayerRequestCommitmentTxToBobClient, err error) {

	requestCreateCommitmentTx := &bean.PayerRequestCommitmentTxOfP2p{}
	_ = json.Unmarshal([]byte(data), requestCreateCommitmentTx)

	retData = &bean.PayerRequestCommitmentTxToBobClient{}
	retData.PayerRequestCommitmentTxOfP2p = *requestCreateCommitmentTx

	tx, err := user.Db.Begin(true)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer tx.Rollback()

	channelInfo := getChannelInfoByChannelId(tx, retData.ChannelId, user.PeerId)
	if channelInfo == nil {
		return nil, errors.New("not found channelInfo at targetSide")
	}

	channelInfo.CurrState = dao.ChannelState_NewTx
	_ = tx.Update(channelInfo)

	senderPeerId := channelInfo.PeerIdA
	if user.PeerId == channelInfo.PeerIdA {
		senderPeerId = channelInfo.PeerIdB
	}
	messageHash := messageService.saveMsgUseTx(tx, senderPeerId, user.PeerId, data)
	retData.MsgHash = messageHash

	commitmentTxInfo, _ := getLatestCommitmentTxUseDbTx(tx, channelInfo.ChannelId, user.PeerId)
	if commitmentTxInfo.Id == 0 {
		commitmentTxInfo.Owner = user.PeerId
		commitmentTxInfo.AmountToRSMC = 0
		commitmentTxInfo.AmountToCounterparty = channelInfo.Amount
	}
	sendChannelStateToTracker(*channelInfo, *commitmentTxInfo)

	_ = tx.Commit()

	return retData, nil
}

func (this *commitmentTxSignedManager) RevokeAndAcknowledgeCommitmentTransaction(msg bean.RequestMessage, signer *bean.User) (retData *bean.PayeeSignCommitmentTxOfP2p, targetUser string, err error) {
	if tool.CheckIsString(&msg.Data) == false {
		err = errors.New(enum.Tips_common_empty + "msg.data")
		log.Println(err)
		return nil, "", err
	}

	reqData := &bean.PayeeSendSignCommitmentTx{}
	err = json.Unmarshal([]byte(msg.Data), reqData)
	if err != nil {
		log.Println(err)
		return nil, "", err
	}

	if tool.CheckIsString(&reqData.MsgHash) == false {
		err = errors.New(enum.Tips_common_wrong + "msg_hash")
		log.Println(err)
		return nil, "", err
	}

	tx, err := signer.Db.Begin(true)
	if err != nil {
		log.Println(err)
		return nil, "", err
	}
	defer tx.Rollback()
	//region 确认是给自己的信息
	message, err := messageService.getMsgUseTx(tx, reqData.MsgHash)
	if err != nil {
		return nil, "", errors.New(enum.Tips_common_invilidMsgHash)
	}
	if message.Receiver != signer.PeerId {
		return nil, "", errors.New(enum.Tips_rsmc_notTargetUser)
	}

	aliceDataJson := &bean.PayerRequestCommitmentTxOfP2p{}
	_ = json.Unmarshal([]byte(message.Data), aliceDataJson)
	reqData.MsgHash = aliceDataJson.CommitmentTxHash
	//endregion

	if tool.CheckIsString(&reqData.ChannelId) == false {
		err = errors.New(enum.Tips_common_empty + "channel_id")
		log.Println(err)
		return nil, "", err
	}

	channelInfo := getChannelInfoByChannelId(tx, reqData.ChannelId, signer.PeerId)
	if channelInfo == nil {
		err = errors.New(enum.Tips_funding_notFoundChannelByChannelId + reqData.ChannelId)
		log.Println(err)
		return nil, "", err
	}
	err = checkBtcFundFinish(channelInfo.ChannelAddress, false)
	if err != nil {
		log.Println(err)
		return nil, "", err
	}

	payeeRevokeAndAcknowledgeCommitment := &dao.PayeeRevokeAndAcknowledgeCommitment{}
	_ = tx.Select(q.Eq("ChannelId", channelInfo.ChannelId), q.Eq("CommitmentTxHash", reqData.MsgHash)).First(payeeRevokeAndAcknowledgeCommitment)
	if payeeRevokeAndAcknowledgeCommitment.Id > 0 {
		return nil, "", errors.New(enum.Tips_rsmc_notDoItAgain)
	}

	//Make sure who creates the transaction, who will sign the transaction.
	//The default creator is Alice, and Bob is the signer.
	//While if ALice is the signer, then Bob creates the transaction.
	targetUser = channelInfo.PeerIdA
	if signer.PeerId == channelInfo.PeerIdB {
		targetUser = channelInfo.PeerIdA
	} else {
		targetUser = channelInfo.PeerIdB
	}

	if targetUser != msg.RecipientUserPeerId {
		return nil, "", errors.New(enum.Tips_rsmc_notTargetUser)
	}

	retData = &bean.PayeeSignCommitmentTxOfP2p{}
	retData.ChannelId = channelInfo.ChannelId
	retData.CommitmentTxHash = reqData.MsgHash
	retData.Approval = reqData.Approval

	if channelInfo.CurrState == dao.ChannelState_NewTx {
		channelInfo.CurrState = dao.ChannelState_CanUse
		_ = tx.Update(channelInfo)
	}

	if reqData.Approval == false {
		payeeRevokeAndAcknowledgeCommitment.ChannelId = retData.ChannelId
		payeeRevokeAndAcknowledgeCommitment.CommitmentTxHash = retData.CommitmentTxHash
		payeeRevokeAndAcknowledgeCommitment.Approval = false
		_ = tx.Save(payeeRevokeAndAcknowledgeCommitment)

		_ = messageService.updateMsgStateUseTx(tx, message)
		err = tx.Commit()
		if err != nil {
			log.Println(err)
			return nil, "", err
		}
		return retData, targetUser, nil
	}

	this.operationFlag.Lock()
	defer this.operationFlag.Unlock()

	//for c rd br
	if tool.CheckIsString(&reqData.ChannelAddressPrivateKey) == false {
		err = errors.New(enum.Tips_common_empty + "channel_address_private_key")
		log.Println(err)
		return nil, "", err
	}

	currNodeChannelPubKey := channelInfo.PubKeyB
	if signer.PeerId == channelInfo.PeerIdA {
		currNodeChannelPubKey = channelInfo.PubKeyA
	}

	if _, err = tool.GetPubKeyFromWifAndCheck(reqData.ChannelAddressPrivateKey, currNodeChannelPubKey); err != nil {
		return nil, "", errors.New(fmt.Sprintf(enum.Tips_rsmc_notPairPrivAndPubKey, reqData.ChannelAddressPrivateKey, currNodeChannelPubKey))
	}
	tempAddrPrivateKeyMap[currNodeChannelPubKey] = reqData.ChannelAddressPrivateKey

	//for rsmc
	if _, err = getAddressFromPubKey(reqData.CurrTempAddressPubKey); err != nil {
		err = errors.New(enum.Tips_common_wrong + "curr_temp_address_pub_key")
		log.Println(err)
		return nil, "", err
	}
	//for rsmc
	if tool.CheckIsString(&reqData.CurrTempAddressPrivateKey) == false {
		err = errors.New(enum.Tips_common_empty + "curr_temp_address_private_key")
		log.Println(err)
		return nil, "", err
	}
	if _, err = tool.GetPubKeyFromWifAndCheck(reqData.CurrTempAddressPrivateKey, reqData.CurrTempAddressPubKey); err != nil {
		return nil, "", errors.New(reqData.CurrTempAddressPrivateKey + " and " + reqData.CurrTempAddressPubKey + " not the pair key")
	}
	tempAddrPrivateKeyMap[reqData.CurrTempAddressPubKey] = reqData.CurrTempAddressPrivateKey

	//for br
	creatorLastTempAddressPrivateKey := aliceDataJson.LastTempAddressPrivateKey
	if tool.CheckIsString(&creatorLastTempAddressPrivateKey) == false {
		err = errors.New(enum.Tips_common_empty + "the starter's last temp address private key")
		log.Println(err)
		return nil, targetUser, err
	}

	// get the funding transaction
	fundingTransaction := getFundingTransactionByChannelId(tx, channelInfo.ChannelId, signer.PeerId)
	if fundingTransaction == nil {
		return nil, "", errors.New(enum.Tips_common_notFound + "fundingTransaction")
	}

	retData.LastTempAddressPrivateKey = reqData.LastTempAddressPrivateKey
	retData.CurrTempAddressPubKey = reqData.CurrTempAddressPubKey

	//region 1、签名对方传过来的rsmcHex
	var signedRsmcHex, aliceRsmcTxId, aliceRsmcMultiAddress, aliceRsmcRedeemScript, aliceRsmcMultiAddressScriptPubKey string
	var aliceRsmcOutputs []rpc.TransactionInputItem
	if tool.CheckIsString(&aliceDataJson.RsmcHex) {
		aliceRsmcTxId, signedRsmcHex, err = rpcClient.BtcSignRawTransaction(aliceDataJson.RsmcHex, reqData.ChannelAddressPrivateKey)
		if err != nil {
			return nil, targetUser, errors.New(fmt.Sprintf(enum.Tips_common_failToSign, "rsmc hex"))
		}
		testResult, err := rpcClient.TestMemPoolAccept(signedRsmcHex)
		if err != nil {
			return nil, targetUser, err
		}
		if gjson.Parse(testResult).Array()[0].Get("allowed").Bool() == false {
			return nil, targetUser, errors.New(gjson.Parse(testResult).Array()[0].Get("reject-reason").String())
		}
		// region 根据alice的临时地址+bob的通道address,获取alice2+bob的多签地址，并得到AliceSignedRsmcHex签名后的交易的input，为创建alice的RD和bob的BR做准备
		aliceMultiAddr, err := rpcClient.CreateMultiSig(2, []string{aliceDataJson.CurrTempAddressPubKey, currNodeChannelPubKey})
		if err != nil {
			return nil, "", err
		}
		aliceRsmcMultiAddress = gjson.Get(aliceMultiAddr, "address").String()
		aliceRsmcRedeemScript = gjson.Get(aliceMultiAddr, "redeemScript").String()
		tempJson, err := rpcClient.GetAddressInfo(aliceRsmcMultiAddress)
		if err != nil {
			return nil, "", err
		}
		aliceRsmcMultiAddressScriptPubKey = gjson.Get(tempJson, "scriptPubKey").String()

		aliceRsmcOutputs, err = getInputsForNextTxByParseTxHashVout(signedRsmcHex, aliceRsmcMultiAddress, aliceRsmcMultiAddressScriptPubKey, aliceRsmcRedeemScript)
		if err != nil {
			log.Println(err)
			return nil, "", err
		}
		retData.SignedRsmcHex = signedRsmcHex
	}

	//endregion

	//endregion

	// region 2、签名对方传过来的 ToCounterpartyTxHex
	_, signedToCounterpartyTxHex, err := rpcClient.BtcSignRawTransaction(aliceDataJson.ToCounterpartyTxHex, reqData.ChannelAddressPrivateKey)
	if err != nil {
		return nil, targetUser, errors.New(fmt.Sprintf(enum.Tips_common_failToSign, "to_counterparty_tx_hex"))
	}
	testResult, err := rpcClient.TestMemPoolAccept(signedToCounterpartyTxHex)
	if err != nil {
		return nil, targetUser, err
	}
	if gjson.Parse(testResult).Array()[0].Get("allowed").Bool() == false {
		return nil, targetUser, errors.New(gjson.Parse(testResult).Array()[0].Get("reject-reason").String())
	}
	retData.SignedToCounterpartyTxHex = signedToCounterpartyTxHex
	//endregion

	//获取bob最新的承诺交易
	latestCommitmentTxInfo, _ := getLatestCommitmentTxUseDbTx(tx, channelInfo.ChannelId, signer.PeerId)
	isFirstRequest := false
	if latestCommitmentTxInfo != nil && latestCommitmentTxInfo.Id > 0 {
		if latestCommitmentTxInfo.TxType != dao.CommitmentTransactionType_Rsmc {
			return nil, "", errors.New(enum.Tips_rsmc_errorCommitmentTxType + strconv.Itoa(int(latestCommitmentTxInfo.TxType)))
		}

		if latestCommitmentTxInfo.CurrState != dao.TxInfoState_CreateAndSign && latestCommitmentTxInfo.CurrState != dao.TxInfoState_Create {
			return nil, "", errors.New(enum.Tips_rsmc_errorCommitmentTxState + strconv.Itoa(int(latestCommitmentTxInfo.CurrState)))
		}

		if latestCommitmentTxInfo.CurrState == dao.TxInfoState_Create && latestCommitmentTxInfo.LastCommitmentTxId > 0 {
			lastCommitmentTx := &dao.CommitmentTransaction{}
			err = tx.Select(q.Eq("Id", latestCommitmentTxInfo.LastCommitmentTxId)).First(lastCommitmentTx)
			if err != nil {
				return nil, "", errors.New(enum.Tips_common_notFound + "lastCommitmentTx")
			}
			_, err = tool.GetPubKeyFromWifAndCheck(reqData.LastTempAddressPrivateKey, lastCommitmentTx.RSMCTempAddressPubKey)
			if err != nil {
				return nil, "", errors.New(fmt.Sprintf(enum.Tips_rsmc_wrongPrivateKeyForLast, reqData.LastTempAddressPrivateKey, lastCommitmentTx.RSMCTempAddressPubKey))
			}
		}

		if latestCommitmentTxInfo.CurrState == dao.TxInfoState_CreateAndSign { //有上一次的承诺交易
			_, err = tool.GetPubKeyFromWifAndCheck(reqData.LastTempAddressPrivateKey, latestCommitmentTxInfo.RSMCTempAddressPubKey)
			if err != nil {
				return nil, "", errors.New(fmt.Sprintf(enum.Tips_rsmc_wrongPrivateKeyForLast, reqData.LastTempAddressPrivateKey, latestCommitmentTxInfo.RSMCTempAddressPubKey))
			}
			isFirstRequest = true
		}
	} else { // 因为没有充值，没有最初的承诺交易C1b
		isFirstRequest = true
	}

	var amountToCounterparty = 0.0
	//如果是本轮的第一次请求交易
	if isFirstRequest {
		//region 3、根据对方传过来的上一个交易的临时rsmc私钥，签名最近的BR交易，保证对方确实放弃了上一个承诺交易
		err := signLastBR(tx, dao.BRType_Rmsc, *channelInfo, signer.PeerId, aliceDataJson.LastTempAddressPrivateKey, latestCommitmentTxInfo.Id)
		if err != nil {
			log.Println(err)
			return nil, "", err
		}
		//endregion

		//region 4、创建C2b
		commitmentTxRequest := &bean.SendRequestCommitmentTx{}
		commitmentTxRequest.ChannelId = channelInfo.ChannelId
		commitmentTxRequest.Amount = aliceDataJson.Amount
		commitmentTxRequest.ChannelAddressPrivateKey = reqData.ChannelAddressPrivateKey
		commitmentTxRequest.CurrTempAddressIndex = reqData.CurrTempAddressIndex
		commitmentTxRequest.CurrTempAddressPubKey = reqData.CurrTempAddressPubKey
		commitmentTxRequest.CurrTempAddressPrivateKey = reqData.CurrTempAddressPrivateKey
		newCommitmentTxInfo, err := createCommitmentTxHex(tx, false, commitmentTxRequest, channelInfo, latestCommitmentTxInfo, *signer)
		if err != nil {
			log.Println(err)
			return nil, "", err
		}
		amountToCounterparty = newCommitmentTxInfo.AmountToCounterparty

		retData.RsmcHex = newCommitmentTxInfo.RSMCTxHex
		retData.ToCounterpartyTxHex = newCommitmentTxInfo.ToCounterpartyTxHex
		//endregion

		// region 5、根据alice的Rsmc，创建对应的BR,为下一个交易做准备，create BR2b tx  for bob
		if len(aliceRsmcOutputs) > 0 {
			var myAddress = channelInfo.AddressB
			if signer.PeerId == channelInfo.PeerIdA {
				myAddress = channelInfo.AddressA
			}
			senderCommitmentTx := &dao.CommitmentTransaction{}
			senderCommitmentTx.Id = newCommitmentTxInfo.Id
			senderCommitmentTx.PropertyId = fundingTransaction.PropertyId
			senderCommitmentTx.RSMCTempAddressPubKey = aliceDataJson.CurrTempAddressPubKey
			senderCommitmentTx.RSMCMultiAddress = aliceRsmcMultiAddress
			senderCommitmentTx.RSMCRedeemScript = aliceRsmcRedeemScript
			senderCommitmentTx.RSMCMultiAddressScriptPubKey = aliceRsmcMultiAddressScriptPubKey
			senderCommitmentTx.RSMCTxHex = signedRsmcHex
			senderCommitmentTx.RSMCTxid = aliceRsmcTxId
			senderCommitmentTx.AmountToRSMC = newCommitmentTxInfo.AmountToCounterparty
			err = createCurrCommitmentTxBR(tx, dao.BRType_Rmsc, channelInfo, senderCommitmentTx, aliceRsmcOutputs, myAddress, reqData.ChannelAddressPrivateKey, *signer)
			if err != nil {
				log.Println(err)
				return nil, "", err
			}
		}
		//endregion
	} else {
		if reqData.CurrTempAddressPubKey != latestCommitmentTxInfo.RSMCTempAddressPubKey {
			return nil, "", errors.New(fmt.Sprintf(enum.Tips_rsmc_notSameValueWhenCreate, reqData.CurrTempAddressPubKey, latestCommitmentTxInfo.RSMCTempAddressPubKey))
		}

		retData.RsmcHex = latestCommitmentTxInfo.RSMCTxHex
		retData.ToCounterpartyTxHex = latestCommitmentTxInfo.ToCounterpartyTxHex
		amountToCounterparty = latestCommitmentTxInfo.AmountToCounterparty
	}

	//region 6、根据签名后的AliceRsmc创建alice的RD create RD tx for alice
	outputAddress := channelInfo.AddressA
	if signer.PeerId == channelInfo.PeerIdA {
		outputAddress = channelInfo.AddressB
	}
	if len(aliceRsmcOutputs) > 0 {
		_, payerRdhex, err := rpcClient.OmniCreateAndSignRawTransactionUseUnsendInput(
			aliceRsmcMultiAddress,
			[]string{
				reqData.ChannelAddressPrivateKey,
			},
			aliceRsmcOutputs,
			outputAddress,
			channelInfo.FundingAddress,
			channelInfo.PropertyId,
			amountToCounterparty,
			getBtcMinerAmount(channelInfo.BtcAmount),
			1000,
			&aliceRsmcRedeemScript)
		if err != nil {
			log.Println(err)
			return nil, targetUser, errors.New(fmt.Sprintf(enum.Tips_rsmc_failToCreate, "RD raw transacation"))
		}
		retData.PayerRdHex = payerRdhex
	}
	//endregion create RD tx for alice

	_ = messageService.updateMsgStateUseTx(tx, message)
	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return nil, "", err
	}
	return retData, "", err
}

func (this *commitmentTxSignedManager) AfterAliceSignCommitmentTransactionAtBobSide(data string, user *bean.User) (retData interface{}, err error) {
	jsonObj := gjson.Parse(data)

	var channelId = jsonObj.Get("channelId").String()
	var signedRsmcHex = jsonObj.Get("signedRsmcHex").String()
	var signedToCounterpartyTxHex = jsonObj.Get("signedToCounterpartyTxHex").String()
	var rdHex = jsonObj.Get("rdHex").String()
	var payerCommitmentTxHash = jsonObj.Get("payerCommitmentTxHash").String()

	tx, err := user.Db.Begin(true)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer tx.Rollback()

	channelInfo := getChannelInfoByChannelId(tx, channelId, user.PeerId)
	if channelInfo == nil {
		return nil, errors.New("not found channelInfo at targetSide")
	}

	latestCommitmentTxInfo, err := getLatestCommitmentTxUseDbTx(tx, channelId, user.PeerId)
	if err != nil {
		err = errors.New("fail to find payer's commitmentTxInfo")
		log.Println(err)
		return nil, err
	}

	if latestCommitmentTxInfo.CurrState != dao.TxInfoState_Create {
		err = errors.New("wrong commitmentTxInfo state " + strconv.Itoa(int(latestCommitmentTxInfo.CurrState)))
		log.Println(err)
		return nil, err
	}

	myChannelAddress := channelInfo.AddressB
	if user.PeerId == channelInfo.PeerIdA {
		myChannelAddress = channelInfo.AddressA
	}

	if tool.CheckIsString(&signedRsmcHex) {
		decodeRsmcHex, err := rpcClient.OmniDecodeTransaction(signedRsmcHex)
		if err != nil {
			return nil, err
		}
		latestCommitmentTxInfo.RSMCTxHex = signedRsmcHex
		latestCommitmentTxInfo.RSMCTxid = gjson.Get(decodeRsmcHex, "txid").Str
		err = signRdTx(tx, channelInfo, signedRsmcHex, rdHex, latestCommitmentTxInfo, myChannelAddress, user)
		if err != nil {
			return nil, err
		}
	}

	if tool.CheckIsString(&signedToCounterpartyTxHex) {
		decodeSignedToOtherHex, err := rpcClient.OmniDecodeTransaction(signedToCounterpartyTxHex)
		if err != nil {
			return nil, err
		}
		latestCommitmentTxInfo.ToCounterpartyTxHex = signedToCounterpartyTxHex
		latestCommitmentTxInfo.ToCounterpartyTxid = gjson.Get(decodeSignedToOtherHex, "txid").Str
	}

	//更新alice的当前承诺交易
	latestCommitmentTxInfo.CurrState = dao.TxInfoState_CreateAndSign
	latestCommitmentTxInfo.SignAt = time.Now()

	bytes, err := json.Marshal(latestCommitmentTxInfo)
	msgHash := tool.SignMsgWithSha256(bytes)
	latestCommitmentTxInfo.CurrHash = msgHash
	_ = tx.Update(latestCommitmentTxInfo)

	lastCommitmentTxInfo := dao.CommitmentTransaction{}
	err = tx.One("Id", latestCommitmentTxInfo.LastCommitmentTxId, &lastCommitmentTxInfo)
	if err == nil {
		lastCommitmentTxInfo.CurrState = dao.TxInfoState_Abord
		_ = tx.Update(lastCommitmentTxInfo)
	}

	payeeRevokeAndAcknowledgeCommitment := &dao.PayeeRevokeAndAcknowledgeCommitment{}
	payeeRevokeAndAcknowledgeCommitment.ChannelId = channelId
	payeeRevokeAndAcknowledgeCommitment.CommitmentTxHash = payerCommitmentTxHash
	payeeRevokeAndAcknowledgeCommitment.Approval = true
	_ = tx.Save(payeeRevokeAndAcknowledgeCommitment)

	_ = tx.Commit()
	return latestCommitmentTxInfo, nil
}

//创建BR
func createCurrCommitmentTxBR(tx storm.Node, brType dao.BRType, channelInfo *dao.ChannelInfo, commitmentTx *dao.CommitmentTransaction, inputs []rpc.TransactionInputItem,
	outputAddress, channelPrivateKey string, user bean.User) (err error) {
	if len(inputs) == 0 {
		return nil
	}
	breachRemedyTransaction := &dao.BreachRemedyTransaction{}
	_ = tx.Select(
		q.Eq("ChannelId", channelInfo.ChannelId),
		q.Eq("InputTxid", commitmentTx.RSMCTxid),
		q.Eq("Type", brType),
		q.Or(
			q.Eq("PeerIdA", user.PeerId),
			q.Eq("PeerIdB", user.PeerId))).
		First(breachRemedyTransaction)
	if breachRemedyTransaction.Id == 0 {
		breachRemedyTransaction, err = createBRTxObj(user.PeerId, channelInfo, brType, commitmentTx, &user)
		if err != nil {
			log.Println(err)
			return err
		}
		if breachRemedyTransaction.Amount > 0 {
			txid, hex, err := rpcClient.OmniCreateAndSignRawTransactionUseUnsendInput(
				commitmentTx.RSMCMultiAddress,
				[]string{
					channelPrivateKey,
				},
				inputs,
				outputAddress,
				channelInfo.FundingAddress,
				channelInfo.PropertyId,
				breachRemedyTransaction.Amount,
				getBtcMinerAmount(channelInfo.BtcAmount),
				0,
				&commitmentTx.RSMCRedeemScript)
			if err != nil {
				log.Println(err)
				return err
			}
			breachRemedyTransaction.OutAddress = outputAddress
			breachRemedyTransaction.Txid = txid
			breachRemedyTransaction.BrTxHex = hex
			breachRemedyTransaction.CurrState = dao.TxInfoState_Create
			_ = tx.Save(breachRemedyTransaction)
		}
	}
	return nil
}

//对上一个承诺交易的br进行签名
func signLastBR(tx storm.Node, brType dao.BRType, channelInfo dao.ChannelInfo, userPeerId string, lastTempAddressPrivateKey string, lastCommitmentTxid int) (err error) {
	lastBreachRemedyTransaction := &dao.BreachRemedyTransaction{}
	err = tx.Select(
		q.Eq("ChannelId", channelInfo.ChannelId),
		q.Eq("CommitmentTxId", lastCommitmentTxid),
		q.Eq("Type", brType),
		q.Or(
			q.Eq("PeerIdA", userPeerId),
			q.Eq("PeerIdB", userPeerId))).
		OrderBy("CreateAt").
		Reverse().First(lastBreachRemedyTransaction)
	if lastBreachRemedyTransaction != nil && lastBreachRemedyTransaction.Id > 0 {
		inputs, err := getInputsForNextTxByParseTxHashVout(
			lastBreachRemedyTransaction.InputTxHex,
			lastBreachRemedyTransaction.InputAddress,
			lastBreachRemedyTransaction.InputAddressScriptPubKey,
			lastBreachRemedyTransaction.InputRedeemScript)
		if err != nil {
			log.Println(err)
			return errors.New(fmt.Sprintf(enum.Tips_rsmc_failToGetInput, "breachRemedyTransaction"))
		}
		signedBRTxid, signedBRHex, err := rpcClient.OmniSignRawTransactionForUnsend(lastBreachRemedyTransaction.BrTxHex, inputs, lastTempAddressPrivateKey)
		if err != nil {
			return errors.New(fmt.Sprintf(enum.Tips_common_failToSign, "breachRemedyTransaction"))
		}
		result, err := rpcClient.TestMemPoolAccept(signedBRHex)
		if err != nil {
			return errors.New(fmt.Sprintf(enum.Tips_common_failToSign, "breachRemedyTransaction"))
		}
		if gjson.Parse(result).Array()[0].Get("allowed").Bool() == false {
			if gjson.Parse(result).Array()[0].Get("reject-reason").String() != "missing-inputs" {
				return errors.New(gjson.Parse(result).Array()[0].Get("reject-reason").String())
			}
		}
		lastBreachRemedyTransaction.Txid = signedBRTxid
		lastBreachRemedyTransaction.BrTxHex = signedBRHex
		lastBreachRemedyTransaction.SignAt = time.Now()
		lastBreachRemedyTransaction.CurrState = dao.TxInfoState_CreateAndSign
		return tx.Update(lastBreachRemedyTransaction)
	}
	return nil
}

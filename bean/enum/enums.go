package enum

type SendTargetType int

const (
	SendTargetType_SendToNone     SendTargetType = -1
	SendTargetType_SendToAll      SendTargetType = 0
	SendTargetType_SendToSomeone  SendTargetType = 1
	SendTargetType_SendToExceptMe SendTargetType = 2
)

type MsgType int

const (
	MsgType_Error_0      MsgType = 0
	MsgType_UserLogin_1  MsgType = 1
	MsgType_UserLogout_2 MsgType = 2

	MsgType_p2p_ConnectServer_3    MsgType = 3
	MsgType_p2p_SendDataToServer_4 MsgType = 4

	MsgType_Core_GetNewAddress_1001                    MsgType = 1001
	MsgType_Core_GetMiningInfo_1002                    MsgType = 1002
	MsgType_Core_GetNetworkInfo_1003                   MsgType = 1003
	MsgType_Core_SignMessageWithPrivKey_1004           MsgType = 1004
	MsgType_Core_VerifyMessage_1005                    MsgType = 1005
	MsgType_Core_DumpPrivKey_1006                      MsgType = 1006
	MsgType_Core_ListUnspent_1007                      MsgType = 1007
	MsgType_Core_BalanceByAddress_1008                 MsgType = 1008
	MsgType_Core_FundingBTC_1009                       MsgType = 1009
	MsgType_Core_BtcCreateMultiSig_1010                MsgType = 1010
	MsgType_Core_Btc_ImportPrivKey_1011                MsgType = 1011
	MsgType_Core_Omni_Getbalance_1200                  MsgType = 1200
	MsgType_Core_Omni_CreateNewTokenFixed_1201         MsgType = 1201
	MsgType_Core_Omni_CreateNewTokenManaged_1202       MsgType = 1202
	MsgType_Core_Omni_GrantNewUnitsOfManagedToken_1203 MsgType = 1203
	MsgType_Core_Omni_RevokeUnitsOfManagedToken_1204   MsgType = 1204
	MsgType_Core_Omni_ListProperties_1205              MsgType = 1205
	MsgType_Core_Omni_GetTransaction_1206              MsgType = 1206
	MsgType_Core_Omni_GetProperty_1207                 MsgType = 1207

	MsgType_Core_Omni_FundingAsset_2001 MsgType = 2001

	MsgType_GetMnemonic_101                MsgType = 101
	MsgType_Mnemonic_CreateAddress_N200    MsgType = -200
	MsgType_Mnemonic_GetAddressByIndex_201 MsgType = -201

	MsgType_ChannelOpen_N32                   MsgType = -32
	MsgType_ChannelOpen_AllItem_N3201         MsgType = -3201
	MsgType_ChannelOpen_ItemByTempId_N3202    MsgType = -3202
	MsgType_ChannelOpen_Count_N3203           MsgType = -3203
	MsgType_ChannelOpen_DelItemByTempId_N3204 MsgType = -3204
	MsgType_GetChannelInfoByChanId_N3206      MsgType = -3206
	MsgType_GetChannelInfoByChanId_N3207      MsgType = -3207

	MsgType_ChannelAccept_N33 MsgType = -33

	MsgType_FundingCreate_AssetFundingCreated_N34                MsgType = -34
	MsgType_FundingCreate_Asset_ALlItem_N3401                    MsgType = -3401
	MsgType_FundingCreate_Asset_ItemById_N3402                   MsgType = -3402
	MsgType_FundingCreate_Asset_ItemByChannelId_N3403            MsgType = -3403
	MsgType_FundingCreate_Asset_Count_N3404                      MsgType = -3404
	MsgType_FundingCreate_BtcFundingCreated_N3400                MsgType = -3400
	MsgType_FundingCreate_Btc_ALlItem_N3451                      MsgType = -3451
	MsgType_FundingCreate_Btc_ItemById_N3452                     MsgType = -3452
	MsgType_FundingCreate_Btc_ItemByChannelId_N3453              MsgType = -3453
	MsgType_FundingCreate_Btc_RDALlItem_N3461                    MsgType = -3461
	MsgType_FundingCreate_Btc_ItemRDById_N3462                   MsgType = -3462
	MsgType_FundingCreate_Btc_ItemRDByTempChannelId_N3463        MsgType = -3463
	MsgType_FundingCreate_Btc_ItemRDByTempChannelIdAndTxId_N3464 MsgType = -3464

	MsgType_FundingSign_AssetFundingSigned_N35 MsgType = -35
	MsgType_FundingSign_BtcSign_N3500          MsgType = -3500

	MsgType_CommitmentTx_CommitmentTransactionCreated_N351 MsgType = -351
	MsgType_CommitmentTx_ItemsByChanId_N35101              MsgType = -35101
	MsgType_CommitmentTx_ItemById_N35102                   MsgType = -35102
	MsgType_CommitmentTx_Count_N35103                      MsgType = -35103
	MsgType_CommitmentTx_LatestCommitmentTxByChanId_N35104 MsgType = -35104
	MsgType_CommitmentTx_LatestRDByChanId_N35105           MsgType = -35105
	MsgType_CommitmentTx_LatestBRByChanId_N35106           MsgType = -35106
	MsgType_SendBreachRemedyTransaction_N35107             MsgType = -35107
	MsgType_CommitmentTx_AllRDByChanId_N35108              MsgType = -35108
	MsgType_CommitmentTx_AllBRByChanId_N35109              MsgType = -35109

	MsgType_CommitmentTxSigned_RevokeAndAcknowledgeCommitmentTransaction_N352 MsgType = -352

	MsgType_CommitmentTxSigned_ToAliceSign_N353     MsgType = -353
	MsgType_CommitmentTxSigned_SecondToBobSign_N354 MsgType = -354

	MsgType_CloseChannelRequest_N38 MsgType = -38
	MsgType_CloseChannelSign_N39    MsgType = -39

	// 寻路
	MsgType_HTLC_FindPath_N4001             MsgType = -4001
	MsgType_HTLC_Invoice_N4003              MsgType = -4003
	MsgType_HTLC_CreatedRAndHInfoItem_N4002 MsgType = -4002
	MsgType_HTLC_AddHTLC_N40                MsgType = -40
	MsgType_HTLC_AddHTLCSigned_N41          MsgType = -41
	MsgType_HTLC_SignedRAndHInfoList_N4101  MsgType = -4101
	MsgType_HTLC_SignedRAndHInfoItem_N4102  MsgType = -4102
	MsgType_HTLC_GetRFromLCommitTx_N4103    MsgType = -4103
	MsgType_HTLC_GetPathInfoByH_N4104       MsgType = -4104
	MsgType_HTLC_GetRInfoByHOfOwner_N4105   MsgType = -4105
	MsgType_HTLC_PayerSignC3b_N42           MsgType = -42
	MsgType_HTLC_PayeeCreateHTRD1a_N43      MsgType = -43
	MsgType_HTLC_PayerSignHTRD1a_N44        MsgType = -44
	MsgType_HTLC_SendR_N45                  MsgType = -45
	MsgType_HTLC_VerifyR_N46                MsgType = -46
	MsgType_HTLC_SendHerdHex_N47            MsgType = -47
	MsgType_HTLC_SignHedHex_N48             MsgType = -48
	MsgType_HTLC_RequestCloseCurrTx_N49     MsgType = -49
	MsgType_HTLC_CloseSigned_N50            MsgType = -50
	MsgType_HTLC_CloseHtlcRequestSignBR_N51 MsgType = -51
	MsgType_HTLC_CloseHtlcUpdateCnb_N52     MsgType = -52

	//https://github.com/LightningOnOmnilayer/Omni-BOLT-spec/blob/master/OmniBOLT-05-Atomic-Swap-among-Channels.md
	MsgType_Atomic_Swap_N80        MsgType = -80
	MsgType_Atomic_Swap_Accept_N81 MsgType = -81
)

func CheckExist(msgType MsgType) bool {
	switch msgType {
	case MsgType_Error_0:
	case MsgType_UserLogin_1:
	case MsgType_UserLogout_2:
	case MsgType_p2p_ConnectServer_3:
	case MsgType_p2p_SendDataToServer_4:
	case MsgType_Core_GetNewAddress_1001:
	case MsgType_Core_GetMiningInfo_1002:
	case MsgType_Core_GetNetworkInfo_1003:
	case MsgType_Core_SignMessageWithPrivKey_1004:
	case MsgType_Core_VerifyMessage_1005:
	case MsgType_Core_DumpPrivKey_1006:
	case MsgType_Core_ListUnspent_1007:
	case MsgType_Core_BalanceByAddress_1008:
	case MsgType_Core_FundingBTC_1009:
	case MsgType_Core_BtcCreateMultiSig_1010:
	case MsgType_Core_Btc_ImportPrivKey_1011:
	case MsgType_Core_Omni_Getbalance_1200:
	case MsgType_Core_Omni_CreateNewTokenFixed_1201:
	case MsgType_Core_Omni_CreateNewTokenManaged_1202:
	case MsgType_Core_Omni_GrantNewUnitsOfManagedToken_1203:
	case MsgType_Core_Omni_RevokeUnitsOfManagedToken_1204:
	case MsgType_Core_Omni_ListProperties_1205:
	case MsgType_Core_Omni_GetTransaction_1206:
	case MsgType_Core_Omni_GetProperty_1207:
	case MsgType_Core_Omni_FundingAsset_2001:
	case MsgType_GetMnemonic_101:
	case MsgType_Mnemonic_CreateAddress_N200:
	case MsgType_Mnemonic_GetAddressByIndex_201:
	case MsgType_ChannelOpen_N32:
	case MsgType_ChannelOpen_AllItem_N3201:
	case MsgType_ChannelOpen_ItemByTempId_N3202:
	case MsgType_ChannelOpen_Count_N3203:
	case MsgType_ChannelOpen_DelItemByTempId_N3204:
	case MsgType_GetChannelInfoByChanId_N3206:
	case MsgType_GetChannelInfoByChanId_N3207:
	case MsgType_ChannelAccept_N33:
	case MsgType_FundingCreate_AssetFundingCreated_N34:
	case MsgType_FundingCreate_Asset_ALlItem_N3401:
	case MsgType_FundingCreate_Asset_ItemById_N3402:
	case MsgType_FundingCreate_Asset_ItemByChannelId_N3403:
	case MsgType_FundingCreate_Asset_Count_N3404:
	case MsgType_FundingCreate_BtcFundingCreated_N3400:
	case MsgType_FundingCreate_Btc_ALlItem_N3451:
	case MsgType_FundingCreate_Btc_ItemById_N3452:
	case MsgType_FundingCreate_Btc_ItemByChannelId_N3453:
	case MsgType_FundingCreate_Btc_RDALlItem_N3461:
	case MsgType_FundingCreate_Btc_ItemRDById_N3462:
	case MsgType_FundingCreate_Btc_ItemRDByTempChannelId_N3463:
	case MsgType_FundingCreate_Btc_ItemRDByTempChannelIdAndTxId_N3464:
	case MsgType_FundingSign_AssetFundingSigned_N35:
	case MsgType_FundingSign_BtcSign_N3500:
	case MsgType_CommitmentTx_CommitmentTransactionCreated_N351:
	case MsgType_CommitmentTx_ItemsByChanId_N35101:
	case MsgType_CommitmentTx_ItemById_N35102:
	case MsgType_CommitmentTx_Count_N35103:
	case MsgType_CommitmentTx_LatestCommitmentTxByChanId_N35104:
	case MsgType_CommitmentTx_LatestRDByChanId_N35105:
	case MsgType_CommitmentTx_LatestBRByChanId_N35106:
	case MsgType_SendBreachRemedyTransaction_N35107:
	case MsgType_CommitmentTx_AllRDByChanId_N35108:
	case MsgType_CommitmentTx_AllBRByChanId_N35109:
	case MsgType_CommitmentTxSigned_RevokeAndAcknowledgeCommitmentTransaction_N352:
	case MsgType_CommitmentTxSigned_ToAliceSign_N353:
	case MsgType_CommitmentTxSigned_SecondToBobSign_N354:
	case MsgType_CloseChannelRequest_N38:
	case MsgType_CloseChannelSign_N39:
	case MsgType_HTLC_FindPath_N4001:
	case MsgType_HTLC_Invoice_N4003:
	case MsgType_HTLC_CreatedRAndHInfoItem_N4002:
	case MsgType_HTLC_AddHTLC_N40:
	case MsgType_HTLC_AddHTLCSigned_N41:
	case MsgType_HTLC_SignedRAndHInfoList_N4101:
	case MsgType_HTLC_SignedRAndHInfoItem_N4102:
	case MsgType_HTLC_GetRFromLCommitTx_N4103:
	case MsgType_HTLC_GetPathInfoByH_N4104:
	case MsgType_HTLC_GetRInfoByHOfOwner_N4105:
	case MsgType_HTLC_PayerSignC3b_N42:
	case MsgType_HTLC_PayeeCreateHTRD1a_N43:
	case MsgType_HTLC_PayerSignHTRD1a_N44:
	case MsgType_HTLC_SendR_N45:
	case MsgType_HTLC_VerifyR_N46:
	case MsgType_HTLC_SendHerdHex_N47:
	case MsgType_HTLC_SignHedHex_N48:
	case MsgType_HTLC_RequestCloseCurrTx_N49:
	case MsgType_HTLC_CloseSigned_N50:
	case MsgType_HTLC_CloseHtlcRequestSignBR_N51:
	case MsgType_HTLC_CloseHtlcUpdateCnb_N52:
	case MsgType_Atomic_Swap_N80:
	case MsgType_Atomic_Swap_Accept_N81:
		return true
	}
	return false
}

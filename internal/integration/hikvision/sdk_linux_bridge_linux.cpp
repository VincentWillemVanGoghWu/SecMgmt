#include "sdk_linux_bridge.h"

#include <string.h>

#include "../../../third_party/HCNetSDK_Linux64/Header/HCNetSDK.h"

extern "C" int goLinuxHikAlarmCallback(int command, void* pAlarmer, char* pAlarmInfo, unsigned int dwBufLen, void* pUser);

static BOOL CALLBACK bridgeAlarmCallback(LONG lCommand, NET_DVR_ALARMER* pAlarmer, char* pAlarmInfo, DWORD dwBufLen, void* pUser) {
    return goLinuxHikAlarmCallback((int)lCommand, (void*)pAlarmer, pAlarmInfo, (unsigned int)dwBufLen, pUser);
}

extern "C" int hik_setup_sdk_init_paths(const char* sdkPath, const char* cryptoPath, const char* sslPath) {
    NET_DVR_LOCAL_SDK_PATH pathCfg;
    memset(&pathCfg, 0, sizeof(pathCfg));
    strncpy(pathCfg.sPath, sdkPath, sizeof(pathCfg.sPath) - 1);
    if (!NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, &pathCfg)) {
        return 0;
    }
    if (!NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_LIBEAY_PATH, (void*)cryptoPath)) {
        return 0;
    }
    if (!NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SSLEAY_PATH, (void*)sslPath)) {
        return 0;
    }
    return 1;
}

extern "C" int hik_sdk_init(void) {
    return NET_DVR_Init() ? 1 : 0;
}

extern "C" void hik_sdk_cleanup(void) {
    NET_DVR_Cleanup();
}

extern "C" void hik_set_connect_time(unsigned int timeoutMs, unsigned int retryCount) {
    NET_DVR_SetConnectTime((DWORD)timeoutMs, (DWORD)retryCount);
}

extern "C" void hik_set_reconnect(unsigned int intervalMs, int enable) {
    NET_DVR_SetReconnect((DWORD)intervalMs, enable ? TRUE : FALSE);
}

extern "C" int hik_setup_sdk_local_config(void) {
    NET_DVR_LOCAL_GENERAL_CFG cfg;
    memset(&cfg, 0, sizeof(cfg));
    cfg.byExceptionCbDirectly = 1;
    cfg.byAlarmJsonPictureSeparate = 1;
    return NET_DVR_SetSDKLocalCfg(NET_DVR_LOCAL_CFG_TYPE_GENERAL, &cfg) ? 1 : 0;
}

extern "C" int hik_set_alarm_callback(void) {
    return NET_DVR_SetDVRMessageCallBack_V31(bridgeAlarmCallback, NULL) ? 1 : 0;
}

extern "C" int hik_login_v40(const char* ip, unsigned short port, const char* username, const char* password, HikDeviceInfo* outInfo) {
    NET_DVR_USER_LOGIN_INFO loginInfo;
    NET_DVR_DEVICEINFO_V40 info;
    memset(&loginInfo, 0, sizeof(loginInfo));
    memset(&info, 0, sizeof(info));

    strncpy((char*)loginInfo.sDeviceAddress, ip, sizeof(loginInfo.sDeviceAddress) - 1);
    loginInfo.wPort = port;
    strncpy((char*)loginInfo.sUserName, username, sizeof(loginInfo.sUserName) - 1);
    strncpy((char*)loginInfo.sPassword, password, sizeof(loginInfo.sPassword) - 1);
    loginInfo.bUseAsynLogin = 0;
    loginInfo.byLoginMode = 0;
    loginInfo.byHttps = 0;

    LONG userID = NET_DVR_Login_V40(&loginInfo, &info);
    if (userID >= 0 && outInfo != NULL) {
        outInfo->startChan = info.struDeviceV30.byStartChan;
        outInfo->startDChan = info.struDeviceV30.byStartDChan;
    }
    return (int)userID;
}

extern "C" int hik_logout(int userID) {
    return NET_DVR_Logout((LONG)userID) ? 1 : 0;
}

extern "C" int hik_setup_alarm_chan_v41(int userID) {
    NET_DVR_SETUPALARM_PARAM param;
    memset(&param, 0, sizeof(param));
    param.dwSize = sizeof(param);
    param.byLevel = 1;
    param.byAlarmInfoType = 1;
    param.byRetAlarmTypeV40 = 0;
    param.byDeployType = 1;
    return (int)NET_DVR_SetupAlarmChan_V41((LONG)userID, &param);
}

extern "C" int hik_close_alarm_chan_v30(int alarmHandle) {
    return NET_DVR_CloseAlarmChan_V30((LONG)alarmHandle) ? 1 : 0;
}

extern "C" int hik_get_file_by_time(int userID, int channelNo, const HikTime* start, const HikTime* end, const char* outputPath) {
    NET_DVR_TIME startTime;
    NET_DVR_TIME endTime;
    memset(&startTime, 0, sizeof(startTime));
    memset(&endTime, 0, sizeof(endTime));

    if (start != NULL) {
        startTime.dwYear = start->year;
        startTime.dwMonth = start->month;
        startTime.dwDay = start->day;
        startTime.dwHour = start->hour;
        startTime.dwMinute = start->minute;
        startTime.dwSecond = start->second;
    }
    if (end != NULL) {
        endTime.dwYear = end->year;
        endTime.dwMonth = end->month;
        endTime.dwDay = end->day;
        endTime.dwHour = end->hour;
        endTime.dwMinute = end->minute;
        endTime.dwSecond = end->second;
    }

    return (int)NET_DVR_GetFileByTime((LONG)userID, (LONG)channelNo, &startTime, &endTime, (char*)outputPath);
}

extern "C" int hik_playback_control(int handle, unsigned int controlCode, unsigned int inValue, unsigned int* outValue) {
    return NET_DVR_PlayBackControl((LONG)handle, (DWORD)controlCode, (DWORD)inValue, (DWORD*)outValue) ? 1 : 0;
}

extern "C" int hik_stop_get_file(int handle) {
    return NET_DVR_StopGetFile((LONG)handle) ? 1 : 0;
}

extern "C" int hik_capture_jpeg_new(int userID, int channelNo, char* buffer, unsigned int bufferSize, unsigned int* returnedSize) {
    NET_DVR_JPEGPARA param;
    memset(&param, 0, sizeof(param));
    param.wPicSize = 0xFF;
    param.wPicQuality = 0;
    return NET_DVR_CaptureJPEGPicture_NEW((LONG)userID, (LONG)channelNo, &param, buffer, (DWORD)bufferSize, (LPDWORD)returnedSize) ? 1 : 0;
}

extern "C" unsigned int hik_get_last_error(void) {
    return NET_DVR_GetLastError();
}

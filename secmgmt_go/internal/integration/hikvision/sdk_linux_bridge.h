#ifndef SECMGMT_HIK_SDK_LINUX_BRIDGE_H
#define SECMGMT_HIK_SDK_LINUX_BRIDGE_H

typedef struct HikDeviceInfo {
    unsigned char startChan;
    unsigned char startDChan;
} HikDeviceInfo;

typedef struct HikTime {
    unsigned int year;
    unsigned int month;
    unsigned int day;
    unsigned int hour;
    unsigned int minute;
    unsigned int second;
} HikTime;

#ifdef __cplusplus
extern "C" {
#endif

int hik_setup_sdk_init_paths(const char* sdkPath, const char* cryptoPath, const char* sslPath);
int hik_sdk_init(void);
void hik_sdk_cleanup(void);
void hik_set_connect_time(unsigned int timeoutMs, unsigned int retryCount);
void hik_set_reconnect(unsigned int intervalMs, int enable);
int hik_setup_sdk_local_config(void);
int hik_set_alarm_callback(void);
int hik_login_v40(const char* ip, unsigned short port, const char* username, const char* password, HikDeviceInfo* outInfo);
int hik_logout(int userID);
int hik_setup_alarm_chan_v41(int userID);
int hik_close_alarm_chan_v30(int alarmHandle);
int hik_get_file_by_time(int userID, int channelNo, const HikTime* start, const HikTime* end, const char* outputPath);
int hik_playback_control(int handle, unsigned int controlCode, unsigned int inValue, unsigned int* outValue);
int hik_stop_get_file(int handle);
int hik_capture_jpeg_new(int userID, int channelNo, char* buffer, unsigned int bufferSize, unsigned int* returnedSize);
unsigned int hik_get_last_error(void);

#ifdef __cplusplus
}
#endif

#endif

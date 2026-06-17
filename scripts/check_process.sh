#!/bin/sh
# */1 * * * * /www/lightMonitor/check_process.sh >> /www/lightMonitor/log/check_process_run.log 2>&1

CheckProcess()
{   
    # 过滤掉 "grep" 和当前脚本 "check_process.sh"
    PROCESS_NUM=`ps -ef | grep "$1" | grep -v "grep" | grep -v "check_process.sh" | wc -l`
    if [ $PROCESS_NUM -ge 1 ];
    then
        return 1 # 进程存在
    else
        return 0 # 进程不存在
    fi
}

# 调用方式：
# pid=$(GetProcessId "lightMonitor")
GetProcessId()
{  
    pid=`ps -ef | grep "$1" | grep -v "grep" | grep -v "check_process.sh" | awk '{print $2}'`
    echo "$pid"
}

CheckProcess "lightMonitor"
check1=$?
if [ $check1 -eq 0 ];
then
    cd /www/lightMonitor || exit 1
    nohup /www/lightMonitor/lightMonitor >/dev/null 2>&1 &
fi

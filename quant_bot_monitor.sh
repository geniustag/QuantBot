#! /bin/bash
result=`ps -ef |grep QuantBot.go|grep -v grep`
dir=/home/deploy/gocode/src/github.com/geniustag/QuantBot
echo `date "+%Y%m%d%H%M%S"`
if [ "$result" != "" ]
then
    echo "QuantBot Running Ok" # >> $dir/nohup.out
else
    echo "QuantBot Crashed" # >> $dir/nohup.out
    export GOPATH="/home/deploy/gocode"
    echo $GOPATH
    cd $dir && nohup /usr/local/go/bin/go run QuantBot.go & # $dir/nohup.out 2>&1
    echo "QuantBot Restarted" #>> $dir/nohup.out
    # echo "...." > nohup.out
fi

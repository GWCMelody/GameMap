#!/bin/bash
while :
do
    for playerNum in {1..50}
    do
	port=$(( 60000+playerNum ))
	./send.sh "loc:${playerNum}.00000:${playerNum}.00000:" "$port"
    done

    sleep 1
done
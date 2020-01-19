#!/bin/bash
#rsync only syncs testServer by default because stableServer shouldn't be edited frequently
rsync -avz --progress -e 'ssh -p 22' testServer/ local\
@73.189.41.182:/home/local/testServer/
echo "test synced"

if [ "$1" = "stable" ]; then
    rsync -avz --progress -e 'ssh -p 22' stableServer/ local\
@73.189.41.182:/home/local/stableServer/
    echo "stable synced"
fi
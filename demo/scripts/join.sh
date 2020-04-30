#!/bin/bash

N=${1:-5}
FASTSYNC=${2:-false}
WEBRTC=${3:-false}
DEST=${4:-"$PWD/conf"}

dest=$DEST/node$N

# Create new key-pair and place it in new conf directory
mkdir -p $dest
echo "Generating key pair for node$N"
docker run  \
    -u $(id -u) \
    -v $dest:/.babble \
    --rm mosaicnetworks/babble:latest keygen 

# copy signal TLS certificate
cp $PWD/../src/net/signal/wamp/test_data/cert.pem $dest/cert.pem

# get genesis.peers.json
echo "Fetching peers.genesis.json from node1"
curl -s http://172.77.5.1:80/genesispeers > $dest/peers.genesis.json

# get up-to-date peers.json
echo "Fetching peers.json from node1"
curl -s http://172.77.5.1:80/peers > $dest/peers.json

# start the new node
docker run -d --name=client$N --net=babblenet --ip=172.77.10.$N -it mosaicnetworks/dummy:latest \
    --name="client $N" \
    --client-listen="172.77.10.$N:1339" \
    --proxy-connect="172.77.5.$N:1338" \
    --discard \
    --log="debug" 

docker create --name=node$N --net=babblenet --ip=172.77.5.$N mosaicnetworks/babble:latest run \
    --heartbeat=100ms \
    --moniker="node$N" \
    --cache-size=50000 \
    --listen="172.77.5.$N:1337" \
    --proxy-listen="172.77.5.$N:1338" \
    --client-connect="172.77.10.$N:1339" \
    --service-listen="172.77.5.$N:80" \
    --fast-sync=$FASTSYNC \
    --log="debug" \
    --sync-limit=100 \
    --webrtc=$WEBRTC \
    --signal-addr="172.77.15.1:2443"

 # --store \

docker cp $dest node$N:/.babble
docker start node$N
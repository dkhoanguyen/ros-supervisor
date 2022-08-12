#!/bin/bash

VERSION=$1

docker build -f dockerfiles/Dockerfile \
            -t ros_supervisor:$VERSION \
            --force-rm \
            .

docker network create --driver=bridge \
                      --subnet=172.21.0.0/16 \
                      --gateway=172.21.0.1 \
                      supervisor

docker run --name ros_supervisor \
           --hostname ros_supervisor \
           --env HOSTMACHINE_HOSTNAME=$HOSTNAME \
           --privileged \
           --restart always \
           --expose 8080 \
           --publish 127.0.0.1:8080:8080 \
           --volume /var/run/docker.sock:/var/run/docker.sock \
           --volume /etc/hosts:/tmp/etc/hosts:rw \
           --volume supervisor:/supervisor \
           --sysctl net.ipv4.ping_group_range='0 2147483647'\
           --network=supervisor --ip=172.21.0.2 \
           --memory 50MB \
           -itd \
           ros_supervisor:$VERSION
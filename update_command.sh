#!/bin/bash

IP_ADDRESS=$1

if [ "$IP_ADDRESS" = "localhost" ]; then
    echo "Development"
    docker cp ros-supervisor.yml ros_supervisor:/supervisor/project/
    docker cp docker-compose.yml ros_supervisor:/supervisor/project/
else
    scp ros-supervisor.yml root@$IP_ADDRESS:. && ssh root@$IP_ADDRESS "docker cp ros-supervisor.yml ros_supervisor:/supervisor/project/"
    scp docker-compose.yml root@$IP_ADDRESS:. && ssh root@$IP_ADDRESS "docker cp docker-compose.yml ros_supervisor:/supervisor/project/"
fi

curl http://$IP_ADDRESS:8080/cmd --include --header "Content-Type: application/json" --request "POST" --data '{"update_core": true, "update_services": true}'

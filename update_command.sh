
docker cp ros-supervisor.yml f0b2b5e940d7:/supervisor/project/ros-supervisor.yml
docker cp docker-compose.yml f0b2b5e940d7:/supervisor/project/docker-compose.yml

curl http://172.21.0.2:8080/cmd --include --header "Content-Type: application/json" --request "POST" --data '{"update": true}'

# curl http://172.20.0.2:8080/health/liveness --silent --include --header "Content-Type: application/json" --request "GET"
# res=$?
# if test "$res" != "0"; then
#    echo "the curl command failed with: $res"
# fi
# echo "the curl command failed with: $res"
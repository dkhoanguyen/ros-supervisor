
docker cp ros-supervisor.yml ros_supervisor:/supervisor/project/ros-supervisor.yml
docker cp ./project/docker-compose.yml ros_supervisor:/supervisor/project/docker-compose.yml

curl http://172.21.0.2:8080/cmd --include --header "Content-Type: application/json" --request "POST" --data '{"update_core": false, "update_services": true}'
# curl http://192.168.0.101:8080/cmd --include --header "Content-Type: application/json" --request "POST" --data '{"update_core": false, "update_services": false}'

# curl http://172.20.0.2:8080/health/liveness --silent --include --header "Content-Type: application/json" --request "GET"
# res=$?
# if test "$res" != "0"; then
#    echo "the curl command failed with: $res"
# fi
# echo "the curl command failed with: $res"
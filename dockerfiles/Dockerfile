# The container that we build the project in
FROM golang:1.17-buster AS build

RUN apt-get update \
  && apt-get install -y --no-install-recommends make git

COPY . /ros-supervisor/
RUN cd /ros-supervisor/ && go mod download 

RUN cd /ros-supervisor/ && go build

RUN mkdir /ros_project

WORKDIR /
CMD [ "bash", "-c", "sysctl -w net.ipv4.ping_group_range='0 2147483647' && \
                     cd /ros-supervisor && ./ros-supervisor" ]
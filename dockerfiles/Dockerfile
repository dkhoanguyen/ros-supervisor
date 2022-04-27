# The container that we build the project in
FROM golang:1.17-buster AS build

RUN apt-get update \
  && apt-get install -y --no-install-recommends make git

COPY . /ros-supervisor/
RUN cd /ros-supervisor/ && go mod download && go build

RUN mkdir /ros_project
RUN sysctl -w net.ipv4.ping_group_range="0 2147483647"

WORKDIR /
CMD [ "bash", "-c", "cd /ros-supervisor && ./ros-supervisor" ]

# FROM scratch as bin

# COPY --from=build /ros-supervisor/ros-supervisor .
# CMD [ "/ros-supervisor" ]
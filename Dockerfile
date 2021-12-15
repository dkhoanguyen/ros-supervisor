# The container that we build the project in
FROM golang:1.17 AS build

RUN apt-get update \
  && apt-get install -y --no-install-recommends make git docker.io

COPY . /ros-supervisor/
RUN cd /ros-supervisor/ && go mod download

CMD ["bash","-c","cd /ros-supervisor/ && ./ros-supervisor"]
# The container that we build the project in
FROM golang:1.17-alpine AS build

WORKDIR /

RUN apk add build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /ros-supervisor

RUN mkdir /ros_project
RUN mkdir /ros_supervisor

# The container that the project runs in
FROM alpine:3.14 AS bin

COPY --from=build /ros_project /ros_project
COPY --from=build /ros-supervisor /ros-supervisor

COPY --from=build /.env.local /.env.local

WORKDIR /

ENTRYPOINT [ "/ros-supervisor" ]
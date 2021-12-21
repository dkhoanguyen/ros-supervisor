BINARY_NAME=ros-supervisor

build:
 	go build

run:
	./${BINARY_NAME}

build_and_run: build run

# clean:
#  go clean
#  rm ${BINARY_NAME}-darwin
#  rm ${BINARY_NAME}-linux
#  rm ${BINARY_NAME}-windows
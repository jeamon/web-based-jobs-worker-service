#== FROM golang:1.17-alpine
FROM golang:alpine

LABEL maintainer="Jerome Amon <https://blog.cloudmentor-scale.com>"
LABEL build_date="2021-08-20"

#== Create an /app directory within image to hold source fils
RUN mkdir /app

#== Setting up working directory
WORKDIR /app

#== Copy the dep files to the container’s workspace.
# or to copy all COPY . /app
COPY go.mod ./

#== Download and install all dependencies
RUN go mod download

#== Copy the local package files to the container’s workspace.
COPY worker.go ./
COPY help.go ./

#== Build the worker program inside the container.
RUN go build -o worker .

#== service is listening on port 8080 exposed to outside world.
EXPOSE 8080

#== install useful curl tool for local testing
# RUN apk --no-cache add curl

#== To be executed once container initialized
RUN chmod +x ./worker

#== not useful since as soon as the shell exists - worker dies
#CMD ["/app/worker", "start"]

# tip to keep the container up & running
# CMD tail -f /dev/null

# [1] docker build --tag unix-worker .
# [2] create and start the container. sleep command added to keep the container up.
# docker run -d --publish 8080:8080 --name unix-worker --rm unix-worker /bin/sh -c "/app/worker start && sleep infinity"
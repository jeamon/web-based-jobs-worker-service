FROM golang:1.17-alpine as builder

LABEL maintainer="Jerome Amon <https://blog.cloudmentor-scale.com/contact>"
LABEL build_date="2021-08-20"

# Setup the working directory
WORKDIR /app/web-worker-service

# Copy go mod file and download dependencies.
COPY go.* ./
RUN go mod download

# Copy all files to the container’s workspace.
COPY . .

# Build the worker program inside the container.
RUN go build -o /app/web-worker-service/worker .

# Copy previous created binary & just the static file then add them
# into your container. Below is a demo example with linux alpine image.
# For debbuging purpose, you can also add useful tool such as curl.

FROM alpine:latest

COPY --from=builder /app/web-worker-service/worker /app/web-worker-service/worker

# Install the useful curl tool for local testing
RUN apk --no-cache add curl

# Make <worker> file executable
RUN chmod +x /app/web-worker-service/worker

# Service is listening on port 8080 exposed to outside world.
EXPOSE 8080

# Tip to keep the container up & running
# CMD tail -f /dev/null

############### INSTRUCTIONS TO RUN IT INTO A DOCKER ################
# [1] docker build --tag web-worker-service .
# [2] create & start the container without volume. sleep command added to keep the container up.
# docker run -d --publish 8080:8080 --name web-worker-service --rm web-worker-service /bin/sh -c "/app/web-worker-service/worker start && sleep infinity"
# [3] you can get inside the container to edit the <worker.config.json> file and restart the worker service.
# [4] create & start the container with volume. sleep command added to keep the container up.
# docker run -d --publish 8080:8080 -v <host-directory-path>:/app/web-worker-service --name web-worker-service --rm web-worker-service /bin/sh -c "/app/web-worker-service/worker start && sleep infinity"
# the <--rm> option will trigger the deletion of the container once it stops or exits.
# to view the container log, you can use the following <docker logs -t web-worker-service> command.
#####################################################################
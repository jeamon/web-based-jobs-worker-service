### BUILD THE IMAGE

```bash
root@ubuntu-server:/home/jeamon/go-projects/web-based-jobs-worker-service# docker build --tag web-worker-service .
Sending build context to Docker daemon  24.37MB
Step 1/14 : FROM golang:1.17-alpine as builder
 ---> 35cd8c8897b1
Step 2/14 : LABEL maintainer="Jerome Amon <https://blog.cloudmentor-scale.com/contact>"
 ---> Using cache
 ---> bd7750efa501
Step 3/14 : LABEL build_date="2021-08-20"
 ---> Using cache
 ---> 7a53b9fde366
Step 4/14 : WORKDIR /app/web-worker-service
 ---> Using cache
 ---> a8799668523d
Step 5/14 : COPY go.* ./
 ---> Using cache
 ---> 03a7717d8ec9
Step 6/14 : RUN go mod download
 ---> Using cache
 ---> de22274d4dba
Step 7/14 : COPY . .
 ---> 035fa9a4dbbd
Step 8/14 : RUN go build -o /app/web-worker-service/worker .
 ---> Running in e03fe7e40606
Removing intermediate container e03fe7e40606
 ---> 28d4b9bc6226
Step 9/14 : FROM alpine:latest
 ---> d4ff818577bc
Step 10/14 : COPY --from=builder /app/web-worker-service/worker /app/web-worker-service/worker
 ---> 2d51e35f7ae2
Step 11/14 : COPY --from=builder /app/web-worker-service/websocket.html /app/web-worker-service/websocket.html
 ---> d44de6cf4bc5
Step 12/14 : RUN apk --no-cache add curl
 ---> Running in 61e3f64ef44d
fetch https://dl-cdn.alpinelinux.org/alpine/v3.14/main/x86_64/APKINDEX.tar.gz
fetch https://dl-cdn.alpinelinux.org/alpine/v3.14/community/x86_64/APKINDEX.tar.gz
(1/5) Installing ca-certificates (20191127-r5)
(2/5) Installing brotli-libs (1.0.9-r5)
(3/5) Installing nghttp2-libs (1.43.0-r0)
(4/5) Installing libcurl (7.79.1-r0)
(5/5) Installing curl (7.79.1-r0)
Executing busybox-1.33.1-r2.trigger
Executing ca-certificates-20191127-r5.trigger
OK: 8 MiB in 19 packages
Removing intermediate container 61e3f64ef44d
 ---> 050dbae13933
Step 13/14 : RUN chmod +x /app/web-worker-service/worker
 ---> Running in 9c1a00a7b950
Removing intermediate container 9c1a00a7b950
 ---> 6f91818d42eb
Step 14/14 : EXPOSE 8080
 ---> Running in 1d903b75a633
Removing intermediate container 1d903b75a633
 ---> b82594a14a61
Successfully built b82594a14a61
Successfully tagged web-worker-service:latest
```

```bash
root@ubuntu-server:/home/jeamon/go-projects/web-based-jobs-worker-service# docker images
REPOSITORY              TAG               IMAGE ID       CREATED             SIZE
web-worker-service      latest            b82594a14a61   23 seconds ago      26.6MB
alpine                  latest            d4ff818577bc   4 months ago        5.6MB
```


### RUN WITHOUT VOLUME

**spin up the container and bind on host port 8080.**

```bash
root@ubuntu-server:/home/jeamon/go-projects/web-based-jobs-worker-service# docker run -d --publish 8080:8080 --name web-worker-service web-worker-service /bin/sh -c "/app/web-worker-service/worker start && sleep infinity"
b39879639731610e30b63e914b7e518d5aed0eaebcd276b70d54745e8829d2f0
```

**verify the container is up & running.**

```bash
root@ubuntu-server:/home/jeamon/go-projects/web-based-jobs-worker-service# docker ps
CONTAINER ID   IMAGE                COMMAND                  CREATED              STATUS              PORTS                                       NAMES
b39879639731   web-worker-service   "/bin/sh -c '/app/we…"   About a minute ago   Up About a minute   0.0.0.0:8080->8080/tcp, :::8080->8080/tcp   web-worker-service
```

**stop and delete the container.**

```bash
root@ubuntu-server:/home/jeamon/go-projects/web-based-jobs-worker-service# docker stop  web-worker-service
web-worker-service
root@ubuntu-server:/home/jeamon/go-projects/web-based-jobs-worker-service# docker container rm  web-worker-service
web-worker-service
```


### RUN WITH VOLUME

**created a local folder on host with revelant files.**

```bash
root@ubuntu-server:/home/jeamon# mkdir web-worker-service
root@ubuntu-server:/home/jeamon# cp worker web-worker-service/
root@ubuntu-server:/home/jeamon# cp worker.config.json web-worker-service/
root@ubuntu-server:/home/jeamon# cp websocket.html web-worker-service/
root@ubuntu-server:/home/jeamon# cd web-worker-service/
root@ubuntu-server:/home/jeamon/web-worker-service# ls -la
total 9176
drwxr-xr-x 2 root   root      4096 Oct 25 10:46 .
drwxr-x--- 7 jeamon jeamon    4096 Oct 25 10:45 ..
-rw-r--r-- 1 root   root      2066 Oct 25 10:46 websocket.html
-rw-r--r-- 1 root   root   9376356 Oct 25 10:45 worker
-rw-r--r-- 1 root   root      1480 Oct 25 10:45 worker.config.json
root@ubuntu-server:/home/jeamon/web-worker-service# chmod +x worker
root@ubuntu-server:/home/jeamon/web-worker-service#
```

**run the docker command with the volume option and bind on host port 8080.**

```bash
root@ubuntu-server:/home/jeamon/web-worker-service# docker run -d --publish 8080:8080 -v /home/jeamon/web-worker-service:/app/web-worker-service --name web-worker-service web-worker-service /bin/sh -c "/app/web-worker-service/worker start && sleep infinity"
f1dbaa4b0baba887666b6b3341b34f28fddfaffdd70cbd9544f87aaccc7b78f7
```

**verify the container is up & running.**

```bash
root@ubuntu-server:/home/jeamon/web-worker-service# docker ps
CONTAINER ID   IMAGE                COMMAND                  CREATED         STATUS         PORTS                                       NAMES
f1dbaa4b0bab   web-worker-service   "/bin/sh -c '/app/we…"   8 seconds ago   Up 7 seconds   0.0.0.0:8080->8080/tcp, :::8080->8080/tcp   web-worker-service
root@ubuntu-server:/home/jeamon/web-worker-service#
```

**for each docker run command you can add --rm option to have the container automatically deleted once it stops or exists.**
#!/usr/bin/env bash

bt=$(date '+%Y-%m-%d %H:%M:%S')
gitCommit="$(git rev-list -1 HEAD)"
goVersion="$(go env GOVERSION)"

go build -o worker -a -ldflags "-X 'main.BuildTime=$bt' -X 'main.GitCommit=$gitCommit' -X 'main.APIVersion=1.0.0' -X 'main.WebVersion=1.0.0' -X 'main.Version=1.0.0' -X 'main.TargetOS=$(go env GOOS)' -X 'main.TargetArch=$(go env GOARCH)' -X 'main.GoVersion=$goVersion'" .
chmod +x ./worker
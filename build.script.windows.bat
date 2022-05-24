@echo off
git rev-list -1 HEAD > tempFile && set /P gitCommit=<tempFile
go env GOVERSION > tempFile && set /P goVersion=<tempFile
go env GOOS > tempFile && set /P targetOS=<tempFile
go env GOARCH > tempFile && set /P targetArch=<tempFile
del tempFile
go build -o worker.exe -a -ldflags "-X 'main.BuildTime=%DATE% %TIME:~0,-3%' -X 'main.GitCommit=%gitCommit%' -X 'main.APIVersion=1.0.0' -X 'main.WebVersion=1.0.0' -X 'main.Version=1.0.0' -X 'main.TargetOS=%targetOS%' -X 'main.TargetArch=%targetArch%' -X 'main.GoVersion=%goVersion%'" .
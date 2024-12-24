@echo off
set GOOS=linux
set GOARCH=arm64
go build -o websql main.go
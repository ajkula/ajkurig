@echo off
cd /d "%~dp0..\.."
go run ./cmd/release -config examples/discs/release.json %*
pause

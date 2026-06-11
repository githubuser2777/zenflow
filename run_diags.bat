@echo off
echo === go build === > diag_output.txt
go build -o proxy.exe ./cmd/proxy >> diag_output.txt 2>&1
echo === go test pkg === >> diag_output.txt
go test ./pkg/... -v >> diag_output.txt 2>&1
echo === go test e2e === >> diag_output.txt
go test ./tests/e2e/... -v -timeout 120s >> diag_output.txt 2>&1
echo === go test pkg race === >> diag_output.txt
go test ./pkg/... -v -race >> diag_output.txt 2>&1
echo === go test e2e race === >> diag_output.txt
go test ./tests/e2e/... -v -timeout 120s -race >> diag_output.txt 2>&1

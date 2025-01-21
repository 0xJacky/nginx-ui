pushd ./cmd/gen || exit
go run generate.go -config ../../app.ini
popd || exit

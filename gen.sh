pushd ./cmd/generate || exit
go run generate.go -config ../../app.ini
popd || exit

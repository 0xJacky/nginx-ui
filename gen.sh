# generate gen code
pushd ./cmd/gen || exit
go run generate.go -config ../../app.ini
popd || exit

# generate error definitions
go run cmd/errdef/generate.go -project . -type ts -output ./app/src/constants/errors -ignore-dirs .devcontainer,app,.github

# parse nginx directive indexs
go run cmd/ngx_dir_index/ngx_dir_index.go ./internal/nginx/nginx_directives.json

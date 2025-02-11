# generate gen code
go run cmd/gen/generate.go -config app.ini

# generate error definitions
go run cmd/errdef/generate.go -project . -type ts -output ./app/src/constants/errors -ignore-dirs .devcontainer,app,.github

# parse nginx directive indexs
go run cmd/ngx_dir_index/ngx_dir_index.go ./internal/nginx/nginx_directives.json

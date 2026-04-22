bin := "bin"
image_tag := "whoson-server:latest"
runtime := if os() == "macos" { "container" } else { "docker" }

default: build

build: whoson-cli whoson-server

whoson-cli:
    go build -o {{bin}}/whoson-cli ./cmd/whoson-cli

whoson-server:
    go build -o {{bin}}/whoson-server ./cmd/whoson-server

oui-db:
    curl -fsSLk -o ouiDB.json https://nw-dlcdnet.asus.com/plugin/js/ouiDB.json

image: oui-db
    {{runtime}} build -t {{image_tag}} .

image-run:
    {{runtime}} run --rm -p 8080:8080 \
        -e R_USER="$R_USER" \
        -e R_PASSWORD="$R_PASSWORD" \
        -e ROUTER_URL="${ROUTER_URL:-http://192.168.50.1}" \
        {{image_tag}}

clean:
    rm -rf {{bin}}

# 11th-init

> :warning: **Project under construction**

## The goal of the project

*11th-init* is init process for containers that helps the application to follow the
[11th principle](https://12factor.net/logs) of [The Twelve-Factor App](https://12factor.net/) - application process should print logs into `stdout` and have the log stream captured by the
execution environment to be routed into centralized storage.

*11th-init* intercepts `stdout` and `stderr` and reproduces the streams for the container runtime
to capture, but also streams the logs over the network to application specific storage that
does not depend on centralized log collection being set up on container runtime level.




## Testing

```console
# generate certififcates
wget https://github.com/tsaarni/certyaml/releases/download/v0.4.0/certyaml-linux-amd64.tar.gz
tar zxvf certyaml-linux-amd64.tar.gz
chmod +x certyaml
./certyaml

# run TLS server
socat openssl-listen:9999,fork,reuseaddr,certificate=server.pem,key=server-key.pem,cafile=root.pem stdout

# run tests
go run main.go -cert client.pem -key client-key.pem -ca-cert root.pem -server localhost:9999 -- ./test-echo-loop.sh
go run main.go -cert client.pem -key client-key.pem -ca-cert root.pem -server localhost:9999 -- ./test-grandchild-zombie.py
go run main.go -cert client.pem -key client-key.pem -ca-cert root.pem -server localhost:9999 -- ./test-sigsegv.sh

pgrep main | xargs kill -9
```

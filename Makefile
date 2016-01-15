###
# Dependencies
# []# go get github.com/Sirupsen/logrus
# []# go get "github.com/gorilla/mux
# []# go get github.com/gorilla/mux
# []# go get github.com/gorilla/rpc
# []# go get github.com/streadway/amqp
###
FILES:"*.go"

run: shell -c `go run $(FILES)`

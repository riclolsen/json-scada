module iec61850_server

go 1.26.5

require (
	github.com/dscsystems/go-iec61850 v0.2.5
	github.com/riclolsen/json-scada/src/go-common v0.0.0
	go.mongodb.org/mongo-driver/v2 v2.8.0
)

require (
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace github.com/riclolsen/json-scada/src/go-common => ../../go-common

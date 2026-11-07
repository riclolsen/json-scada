module modbus-go

go 1.26

replace github.com/riclolsen/json-scada/src/go-common => ../go-common

require (
	github.com/otfabric/go-modbus v1.10.7
	github.com/otfabric/go-serial v0.2.1
	github.com/riclolsen/json-scada/src/go-common v0.0.0-00010101000000-000000000000
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78
	go.mongodb.org/mongo-driver/v2 v2.8.0
)

require (
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

module github.com/mas-soft/whatsapp/mdtest

go 1.17

replace github.com/mas-soft/whatsapp => ../

require (
	github.com/mas-soft/whatsapp v0.0.1
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/mdp/qrterminal/v3 v3.0.0
	google.golang.org/protobuf v1.28.1
)

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	go.mau.fi/libsignal v0.0.0-20221015105917-d970e7c3c9cf // indirect
	golang.org/x/crypto v0.0.0-20221012134737-56aed061732a // indirect
	rsc.io/qr v0.2.0 // indirect
)

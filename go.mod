module github.com/cpacia/obcrawler

go 1.14

require (
	github.com/cpacia/openbazaar3.0 v0.0.0-20200930183357-9fd66d4288d7
	github.com/gcash/bchutil v0.0.0-20200228172631-5e1930e5d630
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/gorilla/mux v1.7.3
	github.com/improbable-eng/grpc-web v0.9.1
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-ipns v0.0.2
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/jessevdk/go-flags v1.4.0
	github.com/jinzhu/gorm v1.9.11
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/libp2p/go-libp2p-kad-dht v0.9.0
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	google.golang.org/grpc v1.30.0
	gorm.io/driver/mysql v1.0.2
	gorm.io/driver/postgres v1.0.2
	gorm.io/driver/sqlite v1.1.3
	gorm.io/gorm v1.20.2
)

replace github.com/lightninglabs/neutrino => github.com/lightninglabs/neutrino v0.11.0

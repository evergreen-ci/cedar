module github.com/evergreen-ci/cedar

go 1.16

require (
	github.com/aclements/go-moremath v0.0.0-20210112150236-f10218a38794
	github.com/evergreen-ci/aviation v0.0.0-20220405151811-ff4a78a4297c
	github.com/evergreen-ci/birch v0.0.0-20220401151432-c792c3d8e0eb
	github.com/evergreen-ci/certdepot v0.0.0-20220408180137-e70afe67cc1b
	github.com/evergreen-ci/gimlet v0.0.0-20220906161625-8fae2ee96785
	github.com/evergreen-ci/pail v0.0.0-20220908201135-8a2090a672b7
	github.com/evergreen-ci/poplar v0.0.0-20220405164038-0cfe3198c320
	github.com/evergreen-ci/timber v0.0.0-20211109152550-dca0e0d04672
	github.com/evergreen-ci/utility v0.0.0-20221202215218-c980e8dea464
	github.com/fraugster/parquet-go v0.11.0
	github.com/jpillora/backoff v1.0.0
	github.com/mongodb/amboy v0.0.0-20230104160932-7df48345d788
	github.com/mongodb/anser v0.0.0-20220408164649-99dd61768f4a
	github.com/mongodb/ftdc v0.0.0-20220401165013-13e4af55e809
	github.com/mongodb/grip v0.0.0-20220401165023-6a1d9bb90c21
	github.com/mongodb/jasper v0.0.0-20230104161055-c79ef0111b0e
	github.com/pkg/errors v0.9.1
	github.com/rs/cors v1.8.3
	github.com/stretchr/testify v1.8.1
	github.com/urfave/cli v1.22.12
	go.mongodb.org/mongo-driver v1.11.1
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1 // indirect
	google.golang.org/grpc v1.51.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/fraugster/parquet-go => github.com/julianedwards/parquet-go v0.11.1-0.20220728161747-424e662fc55b

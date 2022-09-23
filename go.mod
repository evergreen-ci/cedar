module github.com/evergreen-ci/cedar

go 1.16

require (
	github.com/aclements/go-moremath v0.0.0-20210112150236-f10218a38794
	github.com/evergreen-ci/aviation v0.0.0-20211123195311-5ddfd75b3753
	github.com/evergreen-ci/birch v0.0.0-20211025210128-7f3409c2b515
	github.com/evergreen-ci/certdepot v0.0.0-20211109153348-d681ebe95b66
	github.com/evergreen-ci/gimlet v0.0.0-20220906161625-8fae2ee96785
	github.com/evergreen-ci/pail v0.0.0-20220908201135-8a2090a672b7
	github.com/evergreen-ci/poplar v0.0.0-20220119144730-b220d71c0330
	github.com/evergreen-ci/timber v0.0.0-20211109152550-dca0e0d04672
	github.com/evergreen-ci/utility v0.0.0-20220404192535-d16eb64796e6
	github.com/fraugster/parquet-go v0.11.0
	github.com/jpillora/backoff v1.0.0
	github.com/mongodb/amboy v0.0.0-20220209145213-c1c572da4472
	github.com/mongodb/anser v0.0.0-20211116195831-fdc43007b59f
	github.com/mongodb/ftdc v0.0.0-20211028165431-67f017692185
	github.com/mongodb/grip v0.0.0-20220401165023-6a1d9bb90c21
	github.com/mongodb/jasper v0.0.0-20220214215554-82e5a72cff6b
	github.com/pkg/errors v0.9.1
	github.com/rs/cors v1.8.2
	github.com/stretchr/testify v1.8.0
	github.com/urfave/cli v1.22.9
	go.mongodb.org/mongo-driver v1.10.1
	golang.org/x/exp v0.0.0-20210220032938-85be41e4509f // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1 // indirect
	google.golang.org/grpc v1.49.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/fraugster/parquet-go => github.com/julianedwards/parquet-go v0.11.1-0.20220728161747-424e662fc55b

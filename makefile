# start project configuration
name := cedar
buildDir := build
packages := $(name) evergreen rest rest-data rest-model units operations model perf rpc rpc-internal benchmarks
orgPath := github.com/evergreen-ci
projectPath := $(orgPath)/$(name)
# end project configuration


# start environment setup
gobin := $(GO_BIN_PATH)
ifeq ($(gobin),)
gobin := go
endif
gopath := $(GOPATH)
gocache := $(abspath $(buildDir)/.cache)
goroot := $(GOROOT)
ifeq ($(OS),Windows_NT)
gocache := $(shell cygpath -m $(gocache))
gopath := $(shell cygpath -m $(gopath))
goroot := $(shell cygpath -m $(goroot))
endif

export GOPATH := $(gopath)
export GOCACHE := $(gocache)
export GOROOT := $(goroot)
# end environment setup


# Ensure the build directory exists, since most targets require it.
$(shell mkdir -p $(buildDir))


.DEFAULT_GOAL := $(name)

# start lint setup targets
lintDeps := $(buildDir)/run-linter $(buildDir)/golangci-lint
$(buildDir)/golangci-lint:
	@curl  --retry 10 --retry-max-time 60 -sSfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(buildDir) v1.30.0 >/dev/null 2>&1
$(buildDir)/run-linter:cmd/run-linter/run-linter.go $(buildDir)/golangci-lint $(buildDir)
	@$(gobin) build -o $@ $<
# end lint setup targets

# benchmark setup targets
$(buildDir)/run-benchmarks:cmd/run-benchmarks/run-benchmarks.go
	$(gobin) build -o $@ $<
# end benchmark setup targets


# start dependency installation tools
#   implementation details for being able to lazily install dependencies
distContents := $(buildDir)/$(name)
testOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).test)
lintOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).lint)
coverageOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage)
coverageHtmlOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage.html)
# end dependency installation tools


# implementation details for building the binary and creating a
# convenient link in the working directory
$(name):$(buildDir)/$(name)
	@[ -e $@ ] || ln -s $<
$(buildDir)/$(name): .FORCE
	$(gobin) build -ldflags "-w -X github.com/evergreen-ci/cedar.BuildRevision=`git rev-parse HEAD`" -o $@ cmd/$(name)/$(name).go
$(buildDir)/generate-points:cmd/generate-points/generate-points.go
	$(gobin) build -o $@ $<
generate-points:$(buildDir)/generate-points
	./$<
$(buildDir)/make-tarball:cmd/make-tarball/make-tarball.go
	@GOOS="" GOARCH="" $(gobin) build -o $@ $<
# end dependency installation tools


# distribution targets and implementation
dist:$(buildDir)/dist.tar.gz
$(buildDir)/dist.tar.gz:$(buildDir)/make-tarball $(distContents)
	./$< --name $@ --prefix $(name) $(foreach item,$(distContents),--item $(item)) --trim $(buildDir)
	tar -tvf $@
# end deploy and distribution targets


# userfacing targets for basic build and development operations
proto:
	@mkdir -p rpc/internal
	protoc --go_out=plugins=grpc:rpc/internal *.proto
lint:$(lintOutput)
test:$(testOutput)
$(buildDir):$(buildDir)/$(name)
.PHONY: benchmark
benchmark:$(buildDir)/run-benchmarks .FORCE
	./$(buildDir)/run-benchmarks $(run-benchmark)
coverage:$(coverageOutput)
coverage-html:$(coverageHtmlOutput)
phony += lint $(buildDir) test coverage coverage-html
.PRECIOUS:$(lintOutput) $(testOutput) $(coverageOutput) $(coverageHtmlOutput)
# end front-ends


# start vendoring configuration
#    begin with configuration of dependencies
vendor-clean:
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/github.com/evergreen-ci/gimlet/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/github.com/jpillora/backoff/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/github.com/stretchr/testify/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/google.golang.org/grpc/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/google.golang.org/genproto/
	rm -rf vendor/github.com/evergreen-ci/certdepot/vendor/github.com/mongodb/anser/
	rm -rf vendor/github.com/evergreen-ci/certdepot/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/evergreen-ci/certdepot/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/evergreen-ci/certdepot/vendor/github.com/stretchr/testify/
	rm -rf vendor/github.com/evergreen-ci/certdepot/vendor/go.mongodb.org/mongo-driver/
	rm -rf vendor/github.com/evergreen-ci/certdepot/vendor/gopkg.in/mgo.v2/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/davecgh/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/pmezard/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/stretchr/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/gopkg.in/yaml.v2/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/go.mongodb.org/mongo-driver/
	rm -rf vendor/github.com/evergreen-ci/pail/vendor/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/evergreen-ci/pail/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/golang/protobuf/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/mongodb/amboy/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/mongodb/ftdc/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/papertrail/go-tail/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/github.com/stretchr/testify/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/golang.org/x/net/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/golang.org/x/sys/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/golang.org/x/text/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/google.golang.org/genproto/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/google.golang.org/grpc/
	rm -rf vendor/github.com/evergreen-ci/poplar/vendor/gopkg.in/yaml.v2/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/github.com/evergreen-ci/aviation/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/github.com/golang/protobuf/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/github.com/stretchr/testify/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/golang.org/x/net/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/golang.org/x/sys/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/golang.org/x/text/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/google.golang.org/genproto/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/google.golang.org/grpc/
	rm -rf vendor/github.com/evergreen-ci/utility/gitignore.go
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/aws/aws-sdk-go
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/davecgh/go-spew
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/evergreen-ci/gimlet
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/evergreen-ci/poplar/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/stretchr/testify
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/tychoish/gimlet/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/urfave/cli/
	rm -rf vendor/github.com/mongodb/amboy/vendor/go.mongodb.org/mongo-driver
	rm -rf vendor/github.com/mongodb/amboy/vendor/golang.org/x/net/
	rm -rf vendor/github.com/mongodb/amboy/vendor/gonum.org/v1/gonum
	rm -rf vendor/github.com/mongodb/amboy/vendor/gopkg.in/mgo.v2
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/evergreen-ci/utility
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/evergreen-ci/birch
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/mongodb/amboy
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/mongodb/ftdc
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/mongodb/grip
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/tychoish/tarjan
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/satori/go.uuid
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/pkg/errors
	rm -rf vendor/github.com/mongodb/anser/vendor/github.com/stretchr
	rm -rf vendor/github.com/mongodb/anser/vendor/go.mongodb.org/mongo-driver
	rm -rf vendor/github.com/mongodb/anser/vendor/gopkg.in/mgo.v2
	rm -rf vendor/github.com/mongodb/ftdc/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/ftdc/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/mongodb/ftdc/vendor/github.com/satori/go.uuid/
	rm -rf vendor/github.com/mongodb/ftdc/vendor/github.com/satori/go.uuid/gss
	rm -rf vendor/github.com/mongodb/ftdc/vendor/github.com/stretchr/testify
	rm -rf vendor/github.com/mongodb/ftdc/vendor/go.mongodb.org/mongo-driver/
	rm -rf vendor/github.com/mongodb/ftdc/vendor/gopkg.in/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/davecgh/go-spew/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/pkg/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/pmezard/go-difflib/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/grip/vendor/golang.org/x/net/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/aviation/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/gimlet
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/golang/protobuf
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/mongodb/amboy
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/mongodb/ftdc/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/mongodb/grip
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/satori/go.uuid
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/urfave/cli
	rm -rf vendor/github.com/mongodb/jasper/vendor/golang.org/x/net/
	rm -rf vendor/github.com/mongodb/jasper/vendor/golang.org/x/sys/
	rm -rf vendor/github.com/mongodb/jasper/vendor/golang.org/x/text/
	rm -rf vendor/github.com/mongodb/jasper/vendor/google.golang.org/genproto/
	rm -rf vendor/github.com/mongodb/jasper/vendor/google.golang.org/grpc/
	rm -rf vendor/github.com/rs/cors/examples/
	rm -rf vendor/github.com/rs/cors/wrapper
	rm -rf vendor/github.com/stretchr/testify/vendor/
	rm -rf vendor/go.mongodb.org/mongo-driver/data/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/github.com/davecgh/go-spew/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/github.com/pkg/errors/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/github.com/stretchr/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/golang.org/x/net/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/golang.org/x/text/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/golang.org/x/sys/
	rm -rf vendor/go.mongodb.org/mongo-driver/vendor/gopkg.in/yaml.v2/
	rm -rf vendor/gopkg.in/mgo.v2/harness/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/google/uuid/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/google/uuid
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/google/uuid/
	rm -rf vendor/github.com/mongodb/jasper/vendor/gopkg.in/mgo.v2
	rm -rf vendor/github.com/mongodb/jasper/vendor/go.mongodb.org/mongo-driver
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/birch/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/aviation/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/certdepot/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/timber/
	rm -rf vendor/github.com/mongodb/jasper/vendor/github.com/evergreen-ci/poplar/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/go.mongodb.org/mongo-driver/
	rm -rf vendor/github.com/evergreen-ci/timber/vendor/gopkg.in/yaml.v2/
	rm -rf vendor/github.com/evergreen-ci/aviation/vendor/google.golang.org/grpc/
	find vendor/ -name "*.gif" -o -name "*.gz" -o -name "*.png" -o -name "*.ico" -o -name "*testdata*" | xargs rm -rf
phony += vendor-clean
# end vendoring tooling configuration


# convenience targets for runing tests and coverage tasks on a
# specific package.
test-%:$(buildDir)/output.%.test
	@grep -s -q -e "^PASS" $<
coverage-%:$(buildDir)/output.%.coverage
	@grep -s -q -e "^PASS" $(buildDir)/output.$*.test
html-coverage-%:$(buildDir)/output.%.coverage.html
	@grep -s -q -e "^PASS" $(buildDir)/output.$*.test
lint-%:$(buildDir)/output.%.lint
	@grep -v -s -q "^--- FAIL" $<
# end convenience targets


# start test and coverage artifacts
#    This varable includes everything that the tests actually need to
#    run.
testTimeout := -timeout=20m
testArgs := -v $(testTimeout)
ifeq (,$(DISABLE_COVERAGE))
testArgs += -cover
endif
ifneq (,$(RACE_DETECTOR))
testArgs += -race
endif
ifneq (,$(RUN_COUNT))
testArgs += -count=$(RUN_COUNT)
endif
ifneq (,$(RUN_TEST))
testArgs += -run='$(RUN_TEST)'
endif
ifneq (,$(SKIP_LONG))
testArgs += -short
endif
# extra dependencies
# test execution and output handlers
$(buildDir)/output.%.test: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) | tee $@
	@! grep -s -q -e "^FAIL" $@ && ! grep -s -q "^WARNING: DATA RACE" $@
$(buildDir)/output.%.coverage: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) -covermode=count -coverprofile $@ | tee $(buildDir)/output.$*.test
	-[ -f $@ ] && $(gobin) tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
$(buildDir)/output.%.coverage.html:$(buildDir)/output.%.coverage
	$(gobin) tool cover -html=$< -o $@
#  targets to generate gotest output from the linter.
# We have to handle the PATH specially for CI, because if the PATH has a different version of Go in it, it'll break.
$(buildDir)/output.%.lint:$(buildDir)/run-linter .FORCE
	@$(if $(GO_BIN_PATH), PATH="$(shell dirname $(GO_BIN_PATH)):$(PATH)") ./$< --output=$@ --lintBin=$(buildDir)/golangci-lint --packages='$*'
#  targets to process and generate coverage reports
# end test and coverage artifacts


# mongodb utility targets
ifeq ($(OS),Windows_NT)
  decompress := 7z.exe x
else
  decompress := tar -zxvf
endif
mongodb/.get-mongodb:
	rm -rf mongodb
	mkdir -p mongodb
	cd mongodb && curl "$(MONGODB_URL)" -o mongodb.tgz && $(decompress) mongodb.tgz && chmod +x ./mongodb-*/bin/*
	cd mongodb && mv ./mongodb-*/bin/* . && rm -rf db_files && rm -rf db_logs && mkdir -p db_files && mkdir -p db_logs
get-mongodb:mongodb/.get-mongodb
	@touch $<
start-mongod:mongodb/.get-mongodb
	./mongodb/mongod --dbpath ./mongodb/db_files --port 27017 --replSet evg --oplogSize 10
	@echo "waiting for mongod to start up"
start-mongod-auth:mongodb/.get-mongodb
	./mongodb/mongod --auth --dbpath ./mongodb/db_files --port 27017 --replSet evg --oplogSize 10
	@echo "starting up mongod with auth"
init-auth:mongodb/.get-mongodb
	./mongodb/mongo --host `./mongodb/mongo --quiet --eval "db.isMaster()['primary']"` cmd/mongo-auth/create_auth_user.js
	@echo "creating MyUserAdmin"
init-rs:mongodb/.get-mongodb
	./mongodb/mongo --eval 'rs.initiate()'
	sleep 30
check-mongod:mongodb/.get-mongodb
	./mongodb/mongo --nodb --eval "assert.soon(function(x){try{var d = new Mongo(\"localhost:27017\"); return true}catch(e){return false}}, \"timed out connecting\")"
	@echo "mongod is up"
# end mongodb targets


# clean and other utility targets
clean:
	rm -rf *.pb.go $(buildDir)
clean-results:
	rm -rf $(buildDir)/output.*
phony += clean
# end dependency targets


# configure phony targets
.FORCE:
.PHONY:$(phony) .FORCE

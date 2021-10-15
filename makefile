# start project configuration
name := cedar
buildDir := build
packages := $(name) evergreen rest rest-data rest-model units operations model perf rpc rpc-internal benchmarks
orgPath := github.com/evergreen-ci
projectPath := $(orgPath)/$(name)
# end project configuration

# start environment setup
gobin := go
ifneq (,$(GOROOT))
gobin := $(GOROOT)/bin/go
endif

gocache := $(GOCACHE)
ifeq (,$(gocache))
gocache := $(abspath $(buildDir)/.cache)
endif
lintCache := $(GOLANGCI_LINT_CACHE)
ifeq (,$(lintCache))
lintCache := $(abspath $(buildDir)/.lint-cache)
endif

ifeq ($(OS),Windows_NT)
gobin := $(shell cygpath $(gobin))
gocache := $(shell cygpath -m $(gocache))
lintCache := $(shell cygpath -m $(lintCache))
export GOPATH := $(shell cygpath -m $(GOPATH))
export GOROOT := $(shell cygpath -m $(GOROOT))
endif

ifneq ($(gocache),$(GOCACHE))
export GOCACHE := $(gocache)
endif
ifneq ($(lintCache),$(GOLANGCI_LINT_CACHE))
export GOLANGCI_LINT_CACHE := $(lintCache)
endif

export GO111MODULE := off
ifneq (,$(RACE_DETECTOR))
# cgo is required for using the race detector.
export CGO_ENABLED=1
else
export CGO_ENABLED=0
endif
# end environment setup

# Ensure the build directory exists, since most targets require it.
$(shell mkdir -p $(buildDir))

.DEFAULT_GOAL := $(name)

# start lint setup targets
lintDeps := $(buildDir)/run-linter $(buildDir)/golangci-lint
$(buildDir)/golangci-lint:
	@curl  --retry 10 --retry-max-time 60 -sSfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(buildDir) v1.40.0 >/dev/null 2>&1
$(buildDir)/run-linter:cmd/run-linter/run-linter.go $(buildDir)/golangci-lint $(buildDir)
	@$(gobin) build -o $@ $<
# end lint setup targets

# benchmark setup targets
$(buildDir)/run-benchmarks:cmd/run-benchmarks/run-benchmarks.go
	$(gobin) build -o $@ $<
# end benchmark setup targets

# start cli and distribution targets
$(name): $(buildDir)/$(name)
	@[ -e $@ ] || ln -s $<
$(buildDir)/$(name): .FORCE
	$(gobin) build -ldflags "-w -X github.com/evergreen-ci/cedar.BuildRevision=`git rev-parse HEAD`" -trimpath -o $@ cmd/$(name)/$(name).go
$(buildDir)/make-tarball:cmd/make-tarball/make-tarball.go
	@GOOS="" GOARCH="" $(gobin) build -o $@ $<
distContents := $(buildDir)/$(name)
dist: $(buildDir)/dist.tar.gz
$(buildDir)/dist.tar.gz: $(buildDir)/make-tarball $(distContents)
	./$< --name $@ --prefix $(name) $(foreach item,$(distContents),--item $(item)) --trim $(buildDir)
	tar -tvf $@
$(buildDir)/generate-points:cmd/generate-points/generate-points.go
	$(gobin) build -o $@ $<
generate-points:$(buildDir)/generate-points
	./$<
# end cli and distribution targets

# start output files
testOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).test)
lintOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).lint)
coverageOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage)
coverageHtmlOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage.html)
.PRECIOUS: $(lintOutput) $(testOutput) $(coverageOutput) $(coverageHtmlOutput)
# end output files

# start basic development targets
proto:
	@mkdir -p rpc/internal
	protoc --go_out=plugins=grpc:rpc/internal *.proto
lint: $(lintOutput)
test: $(testOutput)
compile: $(buildDir)/$(name)
benchmark: $(buildDir)/run-benchmarks .FORCE
	./$(buildDir)/run-benchmarks
coverage: $(coverageOutput)
coverage-html: $(coverageHtmlOutput)
phony += compile lint test coverage coverage-html proto benchmark

# start convenience targets for running tests and coverage tasks on a
# specific package.
test-%:$(buildDir)/output.%.test
	
coverage-%:$(buildDir)/output.%.coverage
	
html-coverage-%:$(buildDir)/output.%.coverage.html
	
lint-%:$(buildDir)/output.%.lint
	
# end convenience targets
# end basic development targets

# start test and coverage artifacts
testArgs := -v -timeout=20m
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
$(buildDir)/output.%.test: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) | tee $@
	@grep -s -q -e "^PASS" $@
$(buildDir)/output.%.coverage: .FORCE
	$(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) -covermode=count -coverprofile $@ | tee $(buildDir)/output.$*.test
	@-[ -f $@ ] && $(gobin) tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
	@grep -s -q -e "^PASS" $(subst coverage,test,$@)
$(buildDir)/output.%.coverage.html:$(buildDir)/output.%.coverage
	$(gobin) tool cover -html=$< -o $@

ifneq (go,$(gobin))
# We have to handle the PATH specially for linting in CI, because if the PATH has a different version of the Go
# binary in it, the linter won't work properly.
lintEnvVars := PATH="$(shell dirname $(gobin)):$(PATH)"
endif
$(buildDir)/output.%.lint: $(lintDeps) .FORCE
	@$(lintEnvVars) ./$< --output=$@ --lintBin=$(buildDir)/golangci-lint --packages='$*'
# end test and coverage artifacts

# start vendoring configuration
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
# end vendoring configuration

# start mongodb targets
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
init-rs:mongodb/.get-mongodb
	./mongodb/mongo --eval 'rs.initiate()'
check-mongod:mongodb/.get-mongodb
	./mongodb/mongo --nodb --eval "assert.soon(function(x){try{var d = new Mongo(\"localhost:27017\"); return true}catch(e){return false}}, \"timed out connecting\")"
	@echo "mongod is up"
phony += get-mongodb start-mongod init-rs check-mongod
# end mongodb targets

# start cleanup targts
clean:
	rm -rf *.pb.go $(buildDir)
clean-results:
	rm -rf $(buildDir)/output.*
phony += clean
# end cleanup targets

# configure phony targets
.FORCE:
.PHONY: $(phony) .FORCE

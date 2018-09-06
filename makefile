# start project configuration
name := sink
buildDir := build
packages := $(name) rest units operations cost model depgraph
orgPath := github.com/evergreen-ci
projectPath := $(orgPath)/$(name)
# end project configuration


# start linting configuration
#   package, testing, and linter dependencies specified
#   separately. This is a temporary solution: eventually we should
#   vendorize all of these dependencies.
lintDeps := github.com/alecthomas/gometalinter
#   include test files and give linters 40s to run to avoid timeouts
lintArgs := --tests --deadline=14m --vendor
#   gotype produces false positives because it reads .a files which
#   are rarely up to date.
lintArgs += --disable="gotype" --disable="gosec" --enable="goimports" --disable="golint"
lintArgs += --skip="build"
#   enable and configure additional linters
lintArgs += --line-length=100 --dupl-threshold=150 --cyclo-over=15
#   the gotype linter has an imperfect compilation simulator and
#   produces the following false postive errors:
lintArgs += --exclude="error: could not import github.com/mongodb/greenbay"
#   some test cases are structurally similar, and lead to dupl linter
#   warnings, but are important to maintain separately, and would be
#   difficult to test without a much more complex reflection/code
#   generation approach, so we ignore dupl errors in tests.
lintArgs += --exclude="warning: duplicate of .*_test.go"
#   go lint warns on an error in docstring format, erroneously because
#   it doesn't consider the entire package.
lintArgs += --exclude="warning: package comment should be of the form \"Package .* ...\""
#   known issues that the linter picks up that are not relevant in our cases
lintArgs += --exclude="file is not goimported" # top-level mains aren't imported
lintArgs += --exclude="error return value not checked .defer.*"
lintArgs += --exclude="\w+Key is unused.*"
lintArgs += --exclude="unused global variable \w+Key"
lintArgs += --exclude=".*unused variable or constant \w+Key"
# end linting configuration


# start dependency installation tools
#   implementation details for being able to lazily install dependencies
gopath := $(shell go env GOPATH)
lintDeps := $(addprefix $(gopath)/src/,$(lintDeps))
srcFiles := makefile $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -name "*_test.go" -not -path "./buildscripts/*" )
testSrcFiles := makefile $(shell find . -name "*.go" -not -path "./$(buildDir)/*")
testOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).test)
coverageOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage)
coverageHtmlOutput := $(foreach target,$(packages),$(buildDir)/output.$(target).coverage.html)
$(gopath)/src/%:
	@-[ ! -d $(gopath) ] && mkdir -p $(gopath) || true
	go get $(subst $(gopath)/src/,,$@)
$(buildDir)/run-linter:cmd/run-linter/run-linter.go $(buildDir)/.lintSetup
	 go build -o $@ $<
$(buildDir)/.lintSetup:$(lintDeps)
	@-$(gopath)/bin/gometalinter --install >/dev/null && touch $@
# end dependency installation tools


# implementation details for building the binary and creating a
# convienent link in the working directory
$(name):$(buildDir)/$(name)
	@[ -e $@ ] || ln -s $<
$(buildDir)/$(name):$(srcFiles)
	 go build -ldflags "-X github.com/evergreen-ci/sink.BuildRevision=`git rev-parse HEAD`" -o $@ cmd/$(name)/$(name).go
# end dependency installation tools


# distribution targets and implementation
dist:$(buildDir)/dist.tar.gz
$(buildDir)/dist.tar.gz:$(buildDir)/$(name)
	tar -C $(buildDir) -czvf $@ $(name)
# end main build


# userfacing targets for basic build and development operations
lint:$(buildDir)/output.lint
build:$(buildDir)/$(name)
test:$(foreach target,$(packages),test-$(target))
coverage:$(coverageOutput)
coverage-html:$(coverageHtmlOutput)
list-tests:
	@echo -e "test targets:" $(foreach target,$(packages),\\n\\ttest-$(target))
phony += lint lint-deps build build-race race test coverage coverage-html list-race list-tests
.PRECIOUS:$(testOutput) $(coverageOutput) $(coverageHtmlOutput)
.PRECIOUS:$(foreach target,$(packages),$(buildDir)/test.$(target))
.PRECIOUS:$(foreach target,$(packages),$(buildDir)/output.$(target).lint)
.PRECIOUS:$(buildDir)/output.lint
# end front-ends


# convenience targets for runing tests and coverage tasks on a
# specific package.
race-%:$(buildDir)/output.%.race
	@grep -s -q -e "^PASS" $< && ! grep -s -q "^WARNING: DATA RACE" $<
test-%:$(buildDir)/output.%.test
	@grep -s -q -e "^PASS" $<
coverage-%:$(buildDir)/output.%.coverage
	@grep -s -q -e "^PASS" $<
html-coverage-%:$(buildDir)/output.%.coverage.html $(buildDir)/output.%.coverage.html
	@grep -s -q -e "^PASS" $<
lint-%:$(buildDir)/output.%.lint
	@grep -v -s -q "^--- FAIL" $<
# end convienence targets


# start vendoring configuration
#    begin with configuration of dependencies
vendor-clean:
	rm -rf vendor/github.com/stretchr/testify/vendor/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/tychoish/gimlet/
	rm -rf vendor/github.com/mongodb/amboy/vendor/gopkg.in/mgo.v2
	rm -rf vendor/github.com/mongodb/amboy/vendor/gonum.org/v1/gonum
	rm -rf vendor/github.com/mongodb/amboy/vendor/golang.org/x/net/
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/stretchr/testify
	rm -rf vendor/github.com/mongodb/amboy/vendor/github.com/davecgh/go-spew
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/davecgh/go-spew/
	rm -rf vendor/github.com/mongodb/grip/vendor/github.com/pmezard/go-difflib/
	rm -rf vendor/github.com/mongodb/grip/vendor/golang.org/x/net/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/stretchr/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/davecgh/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/pmezard/
	rm -rf vendor/github.com/evergreen-ci/gimlet/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/mongodb/grip/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/tychoish/gimlet/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/mongodb/amboy/
	rm -rf vendor/github.com/mongodb/curator/vendor/golang.org/x/net/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/pkg/errors/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/tychoish/bond/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/papertrail
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/stretchr/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/davecgh/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/pmezard/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/urfave/cli/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/evergreen-ci/
	rm -rf vendor/github.com/mongodb/curator/vendor/github.com/ghodss/
	rm -rf vendor/github.com/mongodb/curator/vendor/golang.org/
	rm -rf vendor/github.com/mongodb/curator/greenbay/
	rm -rf vendor/github.com/mongodb/curator/repobuilder/
	rm -rf vendor/github.com/mongodb/curator/operations/
	rm -rf vendor/github.com/mongodb/curator/cmd/
	rm -rf vendor/github.com/mongodb/curator/main/
	rm -rf vendor/gopkg.in/mgo.v2/harness/
	find vendor/ -name "*.gif" -o -name "*.gz" -o -name "*.png" -o -name "*.ico" -o -name "*testdata*" | xargs rm -rf
phony += vendor-clean
# end vendoring tooling configuration


# start test and coverage artifacts
#    This varable includes everything that the tests actually need to
#    run. (The "build" target is intentional and makes these targetsb
#    rerun as expected.)
testTimeout := -timeout=20m
testArgs := -v $(testTimeout)
ifneq (,$(RUN_TEST))
testArgs += -run='$(RUN_TEST)'
endif
ifneq (,$(RUN_COUNT))
testArgs += -count=$(RUN_COUNT)
endif
ifneq (,$(SKIP_LONG))
testArgs += -short
endif
ifneq (,$(DISABLE_COVERAGE))
testArgs += -cover
endif
ifneq (,$(RACE_DETECTOR))
testArgs += -race
endif
# test execution and output handlers
$(buildDir)/output.%.test:.FORCE
	go test $(testArgs) ./$* | tee $@
$(buildDir)/output.%.coverage:.FORCE
	go test $(testArgs) ./$* -covermode=count -coverprofile $@
	@-[ -f $@ ] && go tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
$(buildDir)/output.$(name).test:.FORCE
	go test $(testArgs) ./ | tee $@
$(buildDir)/output.$(name).coverage:.FORCE
	go test $(testArgs) ./ -covermode=count -coverprofile $@
	@-[ -f $@ ] && go tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
$(buildDir)/output.%.coverage.html:$(buildDir)/output.%.coverage
	 go tool cover -html=$< -o $@
#  targets to generate gotest output from the linter.
$(buildDir)/output.%.lint:$(buildDir)/run-linter $(testSrcFiles) .FORCE
	@./$< --output=$@ --lintArgs='$(lintArgs)' --packages='$*'
$(buildDir)/output.lint:$(buildDir)/run-linter .FORCE
	@./$< --output="$@" --lintArgs='$(lintArgs)' --packages="$(packages)"
#  targets to process and generate coverage reports
# end test and coverage artifacts


# clean and other utility targets
clean:
	rm -rf $(lintDeps) $(buildDir)/coverage.* $(name) $(buildDir)/$(name)
phony += clean
# end dependency targets

# configure phony targets
.FORCE:
.PHONY:$(phony) .FORCE

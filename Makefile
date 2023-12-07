BINARY_DIR = bin
BINARY_NAME = lobby
RUN_ARCH = linux/amd64

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOCLEAN = $(GOCMD) clean
GOTOOL = $(GOCMD) tool
GOCOVFN = coverage.out
CGO_ENABLED = 0

# Build flags and arguments
LDFLAGS = -ldflags="-s -w -X main.version=$(version)"
GCFLAGS = -gcflags="-m -l"
MODFLAGS = -mod=readonly
TRIMPATH = -trimpath

# Cross-compilation target architectures
TARGETS = \
	linux/arm64 \
	linux/amd64

.PHONY: build clean run

# check version is set
valver:
    version := v$(shell grep -oE '\[[^]]+\]' CHANGELOG.md  | sed -n '4{s/\[//;s/\]//p}')
    ifeq ($(shell echo $(version) | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$'),$(version))
        $(info Version '$(version)' is in the correct format)
    else
        $(error Version is not in the correct format. Check CHANGELOG.md)
    endif

# go get dependencies
get:
	$(GOGET) .

# Build for all the configured target architectures (TARGETS)
build: valver get $(TARGETS)

$(TARGETS):
	GOOS=$(word 1, $(subst /, ,$@)) GOARCH=$(word 2, $(subst /, ,$@)) CGO_ENABLED=$(CGO_ENABLED)\
		$(GOBUILD) $(LDFLAGS) $(GCFLAGS) $(MODFLAGS) $(TRIMPATH) \
		-o $(BINARY_DIR)/$(BINARY_NAME)-$(word 1, $(subst /, ,$@))-$(word 2, $(subst /, ,$@))

# Clean builds
clean:
	rm -f $(BINARY_DIR)/$(BINARY_NAME)-*

# Build and run
run: build
	./$(BINARY_DIR)/$(BINARY_NAME)-$(word 1, $(subst /, ,$(RUN_ARCH)))-$(word 2, $(subst /, ,$(RUN_ARCH)))

# Includes race testing
test:
	$(GOTEST) -cover -race -failfast

# Includes race testing and coverage report
testIncCovRep:
	$(GOCLEAN) -testcache
	$(GOTEST) -v -cover -race -failfast -coverprofile=$(GOCOVFN)
	@if [ $$? -eq 0 ]; then $(GOTOOL) cover -html=$(GOCOVFN); fi
	rm -rf coverage.out

# Excludes race testing as some testing functions fail on race tests (not the app)
# Higher unit test coverage with this option
testIncCovRepNoRace:
	$(GOCLEAN) -testcache
	$(GOTEST) -v -cover -failfast -coverprofile=$(GOCOVFN)
	@if [ $$? -eq 0 ]; then $(GOTOOL) cover -html=$(GOCOVFN); fi
	rm -rf coverage.out


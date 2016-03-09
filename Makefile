
ifdef BUILDID
LDFLAGS=-ldflags "-X newtopia.BuildId $(BUILDID)"
endif

all: run

run: install
	$(GOPATH)/bin/soft_delete

build: copy
	cd $(GOPATH)/src/soft_delete; GOPATH=$(GOPATH) go build ./...

install: copy
	@echo Ensure you have called \'make updatedeps\'. Proceeding with install.
	GOPATH=$(GOPATH) GOBIN=$(GOPATH)/bin go install $(LDFLAGS) $(GOPATH)/src/soft_delete/soft_delete.go

copy: clean
	cp -R src/soft_delete $(GOPATH)/src/soft_delete;

updatedeps:
	# locally you can use `make copy; go get newtopia/...` Add -t to also fetch test dependencies.
	go list -f '{{join .Deps "\n"}}' ./... \
		| grep -v newtopia \
		| sort -u \
		| xargs go get -f -u -v

clean:
	rm -rf $(GOPATH)/src/soft_delete

test: copy
	GOPATH=$(GOPATH) go test soft_delete/... -v -p 1

format: fmt
fmt:
	go fmt ./src/...


SHELL   := bash
VERSION := $(shell /bin/date +%Y%m%d%H%M%S)-$(shell git rev-parse --short HEAD)
NAME    := soellman/rumours
IMAGE   := ${NAME}:${VERSION}

.PHONY: all test-vendor build deps

all: test-vendor build
container: linux docker
minikube: container deploy

test:
	go test $(shell glide novendor)

test-vendor:
	go test ./...

test-integration:
	go test -tags=integration $(shell glide novendor)

build:
	go build -o rumors

linux:
	GOOS=linux CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags '-s' -o rumours

docker:
	docker build -t ${NAME} -t ${IMAGE} .

deploy:
	helm upgrade -i rumours helm --set image.tag=${VERSION}

push: container
	@if [[ -z $${TRAVIS} ]] || [[ $${TRAVIS_BRANCH} == 'master' && $${TRAVIS_PULL_REQUEST} == 'false' ]]; then \
		echo "Pushing version ${VERSION}"; \
		docker push ${NAME}; \
		docker push ${IMAGE}; \
	else \
		echo "Not pushing docker image ${VERSION} from git branch $${TRAVIS_BRANCH}"; \
	fi; \


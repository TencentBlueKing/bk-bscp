# init the build information
ifdef HASTAG
	GITTAG=${HASTAG}
else
	GITTAG=$(shell git describe --tags --always)
endif

# version
PRO_DIR   = $(shell pwd)
BUILDTIME = $(shell date +%Y-%m-%dT%T%z)
GITHASH   = $(shell git rev-parse HEAD)
VERSION   = $(shell echo ${ENV_BK_BSCP_VERSION})
DEBUG     = $(shell echo ${ENV_BK_BSCP_ENABLE_DEBUG})
PREFIX   ?= $(shell pwd)
GOBIN     = ${PREFIX}/bin/proto
PATH     := ${PREFIX}/bin/proto:${PATH}
swag      = ${PREFIX}/bin/swag
swagger   = ${PREFIX}/bin/swagger
# image repo tag
REPO ?= ""
TAG ?=  $(shell git describe --tags --match='v*' --dirty='.dirty')
# protoc v4.22.0
export PROTOC_VERSION=25.1
SKIP_FRONTEND_BUILD ?= false
GOBUILD=CGO_ENABLED=0 go build -trimpath



# output directory for release package and version for command line
ifeq ("$(VERSION)", "")
	export OUTPUT_DIR = ${PRO_DIR}/build/bk-bscp
	export LDVersionFLAG = "-X github.com/TencentBlueKing/bk-bscp/pkg/version.BUILDTIME=${BUILDTIME} \
    	-X github.com/TencentBlueKing/bk-bscp/pkg/version.GITHASH=${GITHASH} \
    	-X github.com/TencentBlueKing/bk-bscp/pkg/version.GITTAG=${GITTAG} \
		-X github.com/TencentBlueKing/bk-bscp/pkg/version.DEBUG=${DEBUG}"
else
	export OUTPUT_DIR = ${PRO_DIR}/build/bk-bscp-${VERSION}
	export LDVersionFLAG = "-X github.com/TencentBlueKing/bk-bscp/pkg/version.VERSION=${VERSION} \
    	-X github.com/TencentBlueKing/bk-bscp/pkg/version.BUILDTIME=${BUILDTIME} \
    	-X github.com/TencentBlueKing/bk-bscp/pkg/version.GITHASH=${GITHASH} \
    	-X github.com/TencentBlueKing/bk-bscp/pkg/version.DEBUG=${DEBUG}"
endif

include ./scripts/makefile/uname.mk

default: all

.PHONY: init
init:
	@echo Download protoc
	@mkdir -p ${PREFIX}/bin/proto
	@cd ${PREFIX}/bin/proto && \
		rm -rf protoc-*.zip* && \
		wget -q https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \
		unzip -o protoc-${PROTOC_VERSION}-linux-x86_64.zip && \
		mv -f bin/protoc . && \
		rm -rf protoc-*.zip* readme.txt bin include
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.18.1
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.27.1

	@echo Download gotext
	go install golang.org/x/text/cmd/gotext@v0.20.0

.PHONY: tidy
tidy:
	go mod tidy -compat=1.17

pre:
	@echo -e "\e[34;1mBuilding...\n\033[0m"
	mkdir -p ${OUTPUT_DIR}


install: pre
	@echo -e "\e[34;1mPackaging Install Tools...\033[0m"
	mkdir -p ${OUTPUT_DIR}/install/
	@cp -rf ${PRO_DIR}/scripts/install/start_all.sh ${OUTPUT_DIR}/install/
	@cp -rf ${PRO_DIR}/scripts/install/stop_all.sh ${OUTPUT_DIR}/install/
	@echo -e "\e[34;1mPackaging Install Tools Done\n\033[0m"

api: pre
	@echo -e "\e[34;1mPackaging API Docs...\033[0m"
	@mkdir -p ${OUTPUT_DIR}/api/
	@mkdir -p ${OUTPUT_DIR}/api/api-server
	@cp -f api/api-docs/api-server/api/bk_apigw_resources.yml ${OUTPUT_DIR}/api/api-server
	@tar -czf ${OUTPUT_DIR}/api/api-server/zh.tgz -C api/api-docs/api-server/docs zh
	@mkdir -p ${OUTPUT_DIR}/api/feed-server
	@cp -f api/api-docs/feed-server/api/bk_apigw_resources.yml ${OUTPUT_DIR}/api/feed-server
	@tar -czf ${OUTPUT_DIR}/api/feed-server/zh.tgz -C api/api-docs/feed-server/docs zh
	@echo -e "\e[34;1mPackaging API Docs Done\n\033[0m"

pb:
	@echo -e "\e[34;1mFormat proto...\033[0m"
	@find pkg/protocol -type f -name "*.proto"|xargs clang-format -i --style="{BasedOnStyle: Google, ColumnLimit: 120}"
	@echo -e "\e[34;1mMake Protocol...\033[0m"
	@cd pkg/protocol && make clean && make
	@echo -e "\e[34;1mMake Protocol Done\n\033[0m"


api_docs:
	@mkdir -p ${PREFIX}/docs/swagger
	@protoc --proto_path=. --proto_path=internal/thirdparty/protobuf/ \
	--openapiv2_out docs/swagger \
	--openapiv2_opt allow_merge=true \
	--openapiv2_opt preserve_rpc_order=true \
	--openapiv2_opt merge_file_name=api \
	--openapiv2_opt output_format=json \
	--openapiv2_opt visibility_restriction_selectors=INTERNAL \
	--openapiv2_opt visibility_restriction_selectors=BKAPIGW \
	--openapiv2_opt use_go_templates=true pkg/protocol/config-server/*.proto

bkapigw_docs:
	@mkdir -p ${PREFIX}/docs/swagger
	@protoc --proto_path=. --proto_path=internal/thirdparty/protobuf/ \
	--openapiv2_out docs/swagger \
	--openapiv2_opt allow_merge=true \
	--openapiv2_opt preserve_rpc_order=true \
	--openapiv2_opt merge_file_name=bkapigw \
	--openapiv2_opt output_format=json \
	--openapiv2_opt visibility_restriction_selectors=BKAPIGW \
	--openapiv2_opt use_go_templates=true pkg/protocol/config-server/*.proto

.PHONY: gen
gen:
	@go run scripts/gen/main.go

# 与 .github/workflows/lint.yml 中 golangci-lint 版本保持一致
GOLANGCI_LINT_IMAGE ?= golangci/golangci-lint:v2.8.0

.PHONY: lint
lint:
	@docker run --rm -v ${PRO_DIR}:/app -w /app ${GOLANGCI_LINT_IMAGE} \
		sh -c "go mod download && golangci-lint run --issues-exit-code=0 --fix --timeout=5m"

test: pre
	@cd test/suite && make && cp -rf suite-test ${OUTPUT_DIR}/ && rm -rf suite-test
	@cd test/benchmark && make && cp -rf bench-test ${OUTPUT_DIR}/ && rm -rf bench-test

mock: pre
	@cd ${PRO_DIR}/test/mock/repo && make

all: pre validate pb install test api mock build_bscp
	@echo -e "\e[34;1mBuild All Success!\n\033[0m"

server: validate api
	@echo -e "\e[34;1mMaking Server...\n\033[0m"
	@echo "version: ${VERSION}" > ${OUTPUT_DIR}/VERSION
	@cp -rf ${PRO_DIR}/server-CHANGELOG.md ${OUTPUT_DIR}
	@mkdir -p ${OUTPUT_DIR}/install/
	@mkdir -p ${OUTPUT_DIR}
	@mkdir -p ${OUTPUT_DIR}/etc
	@cd ${PRO_DIR}/cmd && make server
	@echo -e "\e[34;1mMake Server All Success!\n\033[0m"

validate:
	@if [ "$(VERSION)" != "" ];then \
		if [[ "$(VERSION)" =~ ^(v[0-9]+.[0-9]+.[0-9]+){1}(-alpha[0-9]+)? ]];then \
			echo "version: "$(VERSION); \
		else \
			exit 1; \
		fi \
  	fi

clean:
	@cd ${PRO_DIR}/cmd && make clean
	@rm -rf ${PRO_DIR}/build

.PHONY: build_bscp
build_bscp:
	@cd ${PRO_DIR}/cmd && make all

.PHONY: build_feed
build_feed:
	@cd ${PRO_DIR}/cmd && make feed

.PHONY: build_vault
build_vault:
	@cd ${PRO_DIR}/cmd && make vault

.PHONY: build_frontend
build_frontend:
	@echo "tips: ensure you have installed node and npm"
	cd ui; npm install --legacy-peer-deps; npm run build

.PHONY: build_ui
build_ui: 
	@echo -e "\e[34;1mTips: ensure you have execute 'make build_frontend' first\033[0m"
	${GOBUILD} -ldflags ${LDVersionFLAG} -o bscp-ui ./cmd/ui

.PHONY: docker
docker:
	@docker build -t bk-bscp-hyper:latest .

.PHONY: i18n
i18n:
	@go generate ./internal/i18n/translations/translations.go
	@cp ./internal/i18n/translations/locales/zh/out.gotext.json ./internal/i18n/translations/locales/zh/messages.gotext.json


${swag}:
	@echo ">> downloading swag"
	@mkdir -p ${PREFIX}/bin
	@wget -q -O ${swag} https://github.com/ifooth/swag/releases/download/v1.16.4-r1/swag && chmod a+x ${swag}

${swagger}:
	@echo ">> downloading swagger"
	@mkdir -p ${PREFIX}/bin
	@wget -q -O ${swagger} https://github.com/ifooth/go-swagger/releases/download/v0.31.0-r1/swagger && chmod a+x ${swagger}

.PHONY: markdown_docs
markdown_docs: ${swag} ${swagger}
	${swag} fmt -d ./cmd
	${swag} init -g ./cmd/api-server/api_server.go  --parseDependency --parseInternal --outputTypes json,json -o ./docs/swagger/apiserver
	# 修正bkapigw的swagger.json 的default值()
	sed -i 's/"default": "false"/"default": false/g' ./docs/swagger/bkapigw.swagger.json
	${swagger} validate ./docs/swagger/bkapigw.swagger.json
	# 合并bkapigw和apiserver的swagger.json
	$(swagger) mixin ./docs/swagger/bkapigw.swagger.json ./docs/swagger/apiserver/swagger.json -o ./docs/swagger/bkapigw/swagger.json
	${swagger} generate markdown  --output=bkapigw_swagger.md -T ./docs/swagger -f ./docs/swagger/bkapigw/swagger.json -t ./docs/swagger/bkapigw

.PHONY: docs
docs: api_docs bkapigw_docs markdown_docs

.PHONY: push-image
push-image: 
	@if [ "${SKIP_FRONTEND_BUILD}" != "true" ]; then \
		echo -e "\e[34;1mBuilding frontend...\033[0m"; \
		$(MAKE) build_frontend; \
	else \
		echo -e "\e[33;1mSkipping frontend build as SKIP_FRONTEND_BUILD=true\033[0m"; \
	fi
	$(MAKE) build_bscp
	docker build -t ${REPO}/bk-bscp-hyper:${TAG} . --push

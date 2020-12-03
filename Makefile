
.PHONY: gen-scaler
gen-scaler:
	protoc scaler/scaler.proto --go_out=plugins=grpc:externalscaler

.PHONY: build-scaler
build-scaler:
	go build -o bin/scaler ./scaler

.PHONY: docker-build-scaler
docker-build-scaler:
	docker build -t arschles/scaler -f scaler/Dockerfile .

.PHONY: build-interceptor
build-interceptor:
	go build -o bin/interceptor ./interceptor

.PHONY: docker-build-interceptor
docker-build-interceptor:
	docker build -t arschles/interceptor -f interceptor/Dockerfile .

.PHONY: build-operator
build-operator:
	cargo build --bin operator

.PHONY: docker-build-operator
docker-build-operator:
	docker build -t arschles/keda-http-operator -f operator/Dockerfile operator

.PHONY: docker-push-operator
docker-push-operator:
	docker push arschles/keda-http-operator

.PHONY: helm-operator
helm-operator:
	helm upgrade \
		--set image.tag=latest \
		--set image.pullPolicy=Always \
		--set image.repository=arschles/keda-http-operator \
		--create-namespace \
		--install \
		--namespace \
		keda-http keda-http-operator ./operator/chart


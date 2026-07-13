CTR=docker
NAME=woom
RUSTBUILD=cargo build --release --manifest-path rust/Cargo.toml

.PHONY: default
default: webapp rust-build

.PHONY: build
build: rust-build

.PHONY: rust-build
rust-build:
	$(RUSTBUILD)
	cp rust/target/release/woom-server $(NAME)

.PHONY: build-go-fallback
build-go-fallback:
	CGO_ENABLED=0 go build -tags release -trimpath -o $(NAME)-go

.PHONY: webapp
webapp:
	npm run build

.PHONY: webapp-clean
webapp-clean:
	rm -r static/dist

.PHONY: clean
clean: webapp-clean
	cargo clean --manifest-path rust/Cargo.toml
	go clean -cache

.PHONY: cli-redis
cli-redis:
	$(CTR) run -it --rm --network=host \
		-e IREDIS_URL=redis://localhost:6379/0 \
		dbcliorg/iredis

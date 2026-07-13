CTR=docker
NAME=woom
RUSTBUILD=cargo build --release --manifest-path rust/Cargo.toml

.PHONY: default
default: frontend rust-build

.PHONY: build
build: rust-build

.PHONY: rust-build
rust-build:
	$(RUSTBUILD)
	cp rust/target/release/woom-server $(NAME)

.PHONY: frontend
frontend:
	npm run build

.PHONY: frontend-clean
frontend-clean:
	rm -r static/dist

.PHONY: clean
clean: frontend-clean
	cargo clean --manifest-path rust/Cargo.toml

.PHONY: cli-redis
cli-redis:
	$(CTR) run -it --rm --network=host \
		-e IREDIS_URL=redis://localhost:6379/0 \
		dbcliorg/iredis

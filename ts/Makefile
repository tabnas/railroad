.PHONY: all build test clean build-ts test-ts clean-ts reset build-rs test-rs clean-rs

all: build test

build: build-ts

test: test-ts

clean: clean-ts

# TypeScript
build-ts:
	npm run build

test-ts:
	npm test

clean-ts:
	rm -rf dist dist-test

reset:
	npm run reset

# Rust (the crate in ../rs; the engine is a sibling checkout)
build-rs:
	cd ../rs && cargo build --all-targets

test-rs:
	cd ../rs && cargo test --all-targets && cargo test --doc
	cd ../rs && cargo clippy --all-targets --all-features -- -D warnings

clean-rs:
	cd ../rs && cargo clean

FROM rust:1.75 as builder
WORKDIR /src
COPY . .
RUN cargo build --release

FROM debian:bookworm-slim
COPY --from=builder /src/target/release/kuberhealthy-example /usr/local/bin/kuberhealthy-example
ENTRYPOINT ["/usr/local/bin/kuberhealthy-example"]

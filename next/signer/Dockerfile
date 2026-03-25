# Build the Smesh Signer browser extension from source.
#
# Usage:
#   docker build -t smesh-signer-build .
#   docker run --rm -v $(pwd)/out:/out smesh-signer-build
#
# Output:
#   out/firefox/              Unpacked extension (load in about:debugging)
#   out/smesh_signer-*.zip    Signed/packaged XPI

FROM oven/bun:1-debian AS builder

WORKDIR /ext

# Install deps first (cache layer)
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source
COPY angular.json tsconfig*.json custom-webpack.config.ts eslint.config.js ./
COPY src/ src/
COPY public/ public/
COPY common/ common/

# Build Firefox extension
RUN bun run build:firefox

# Package XPI
RUN bunx web-ext build --source-dir dist/firefox --artifacts-dir dist --overwrite-dest

# Copy stage: extract built artifacts
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /ext

COPY --from=builder /ext/dist/ /ext/dist/

# Default: copy artifacts to /out (mount a volume)
CMD ["sh", "-c", "cp -r /ext/dist/* /out/ && echo 'Extension built to /out/'"]

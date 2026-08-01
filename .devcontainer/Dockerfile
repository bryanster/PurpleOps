# The development container.
#
# Deliberately the same base as the image the product ships in
# (deploy/Dockerfile's build stage): same Go, same Debian release, same glibc.
# The DuckDB driver is cgo, so "it compiles in my editor" and "it compiles in
# the release image" should not be two different questions.
#
# What is *not* here is as deliberate. golangci-lint and oapi-codegen are not
# baked in: the Makefile pins their versions and installs them into ./bin, and a
# second copy in the image is a second version to drift. `make tools` is the
# postCreateCommand, and it is the same command a developer without a container
# runs.

FROM golang:1.25-trixie

# make: the Makefile is the interface to everything in this repo, and the Go
#   image does not ship it.
# The rest: git over https and ssh, and the handful of things that make a shell
#   in here usable rather than a puzzle.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      git \
      jq \
      less \
      make \
      openssh-client \
      unzip \
 && rm -rf /var/lib/apt/lists/*

# Node, taken from the official image rather than installed from a distribution
# repository, because the version has to be *exact*: .prototools, web/.nvmrc and
# web/package.json's `engines` all name 24.18.1, and so does the node stage in
# deploy/Dockerfile. A NodeSource "24.x" would drift away from that silently.
#
# Copying /usr/local wholesale would bury the Go toolchain that lives there, so
# only Node's own files come across, and the npm/npx entry points are relinked.
COPY --from=node:24.18.1-trixie-slim /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node:24.18.1-trixie-slim /usr/local/bin/node /usr/local/bin/node
RUN ln -sf /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
 && ln -sf /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
 && ln -sf /usr/local/lib/node_modules/corepack/dist/corepack.js /usr/local/bin/corepack \
 && node --version && npm --version

RUN npm install -g @anthropic-ai/claude-code

# Not set in the image: PURPLEOPS_* configuration. The server reads it from the
# environment and refuses anything it cannot use, so a container-wide default
# would be a value nobody chose, quietly in force. Copy .env.example to .env.

# Ubuntu image for the RealBrowser Cursor Cloud hermetic engineering lane.
# Cursor checks out the repository separately; do not COPY the project.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:/usr/local/bin:$PATH \
    COREPACK_ENABLE_DOWNLOAD_PROMPT=0

# Match rust-toolchain.toml: channel 1.97.1, profile minimal, clippy + rustfmt.
ARG RUST_TOOLCHAIN=1.97.1
# Node.js 22 LTS.
ARG NODE_VERSION=22.22.2
# Root package.json packageManager.
ARG PNPM_VERSION=11.20.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        pkg-config \
        curl \
        ca-certificates \
        git \
        file \
        wget \
        xz-utils \
        sudo \
        libssl-dev \
        libwebkit2gtk-4.1-dev \
        libgtk-3-dev \
        libayatana-appindicator3-dev \
        librsvg2-dev \
        patchelf \
        libsoup-3.0-dev \
        libjavascriptcoregtk-4.1-dev \
        libxdo-dev \
    && rm -rf /var/lib/apt/lists/*

# Official Node.js 22 LTS tarball for amd64 and arm64 cloud hosts.
RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
         amd64) node_arch=x64 ;; \
         arm64) node_arch=arm64 ;; \
         *) echo "unsupported architecture: $arch" >&2; exit 1 ;; \
       esac \
    && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" \
         | tar -xJ -C /usr/local --strip-components=1 \
    && corepack enable \
    && corepack prepare "pnpm@${PNPM_VERSION}" --activate \
    && node --version \
    && pnpm --version

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
      | sh -s -- -y --no-modify-path \
          --default-toolchain "${RUST_TOOLCHAIN}" \
          --profile minimal \
          --component clippy \
          --component rustfmt \
    && chmod -R a+rwX "${RUSTUP_HOME}" "${CARGO_HOME}" \
    && rustc --version \
    && rustup component list --installed

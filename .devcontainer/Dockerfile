FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    make \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Node.js 22 for Claude Code
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

# golangci-lint matching project version
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    | sh -s -- -b /usr/local/bin v2.5.0

# pre-commit via standalone .pyz (no pip needed)
RUN curl -fsSL -o /usr/local/bin/pre-commit \
    https://github.com/pre-commit/pre-commit/releases/download/v4.2.0/pre-commit-4.2.0.pyz \
    && chmod +x /usr/local/bin/pre-commit

# mongosh for querying the dev database
RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc \
    | gpg --dearmor -o /usr/share/keyrings/mongodb-server-8.0.gpg \
    && echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg ] \
    https://repo.mongodb.org/apt/debian bookworm/mongodb-org/8.0 main" \
    > /etc/apt/sources.list.d/mongodb-org-8.0.list \
    && apt-get update && apt-get install -y mongodb-mongosh \
    && rm -rf /var/lib/apt/lists/*

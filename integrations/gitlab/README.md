# StageFreight GitLab Integration

Two CI skeleton variants. Copy the one that matches your repo into `.gitlab-ci.yml`.

## `standard.gitlab-ci.yml`

For repos that build artifacts (Docker images, binaries). Includes DinD, build cache, security scanning, release, and docs.

## `lightweight.gitlab-ci.yml`

For repos that don't build artifacts (GitOps IaC, governance/policy, docs-only). No DinD, no build stages. Includes validate, reconcile, and docs.

## `runner-docker-compose.example.yml`

Reference Docker Compose for self-hosted GitLab Runner with persistent DinD and StageFreight cache.

## How it works

All jobs call `stagefreight ci run <subsystem>`. StageFreight reads `.stagefreight.yml` and decides what each subsystem does at runtime. The skeleton is just transport — StageFreight owns all logic.

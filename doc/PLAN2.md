# PLAN MODE ONLY — AWS migration analysis for Bitácora de Plantas

You are in PLAN MODE. DO NOT modify, create, or delete ANY file. Do not run builds.

## Scope (to save time)
Read ONLY these files: main.go, templates/index.html, templates/plant-grid.html, templates/plant-detail.html, docker-compose.yml, Dockerfile, README.md. IGNORE all TASK*.md and PLAN*.md files.

## Deliverable
Your COMPLETE final report must be your LAST message as plain markdown text (do NOT write it to a file). The report is the deliverable — make it thorough and concrete.

## Context
Single-user plant journal: Go stdlib + HTMX v4 + Alpine.js, Docker Compose on a home Raspberry Pi, filesystem storage (data/plants/<slug>/meta.json + images/ + .display/ + .thumb/ JPEG versions, lazy generation, process-local mutex), named Docker volume, Cloudflare tunnel. Goal: production multi-user deployment on AWS.

## Report must cover (in order)
1. **Current internal architecture** — routes/handlers, storage model, image pipeline, concurrency model, Docker setup, and a bullet list of multi-user limitations.
2. **AWS services recommendation** — one service per concern with a one-line rationale: compute (ECS Fargate vs App Runner), ECR, S3 for images, CloudFront, metadata DB (RDS PostgreSQL vs DynamoDB — pick one), Secrets Manager, Cognito auth, VPC/ALB/Route53/ACM, CloudWatch, CI/CD, optional WAF.
3. **File-level code changes** — storage interface to abstract (list the functions), metadata to DB (schema sketch: users/plants/images), per-user scoping, auth middleware + HTMX integration, image pipeline on S3, config via env, multi-instance safety, /health endpoint, graceful shutdown, upload streaming to S3, footer total as aggregate query.
4. **Data migration** — Docker volume → S3 + DB (concrete steps).
5. **Target architecture** — ASCII diagram + phased rollout (MVP → auth/multi-user → hardening) with rough effort notes.
6. **Security checklist** — HTTPS, authz, upload validation, rate limiting, private S3 + OAC, private subnets, backups.

Be specific: name the actual functions from main.go (e.g. saveImages, createThumbnail, handleServeImage, totalImageSize) that need to move behind an interface.

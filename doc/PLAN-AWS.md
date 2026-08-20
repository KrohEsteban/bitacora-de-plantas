# Bitácora de Plantas — AWS Migration Research (PLAN MODE ONLY)

You are in PLAN MODE: analyze and research ONLY. DO NOT modify, create, or delete ANY file. DO NOT run builds or deploys. Produce a report as your final output.

## Context
The app in the current directory is a single-user plant journal: Go (stdlib net/http) + HTMX v4 + Alpine.js, running in Docker Compose on a home Raspberry Pi. Storage: local filesystem under data/plants/<slug>/ (meta.json + images/ + .display/ + .thumb/ derived JPEG versions, lazy generation, imgMu mutex). Data currently lives in a named Docker volume (bitacora-data). No authentication, no database, single instance. It's exposed publicly via a Cloudflare tunnel.

The owner wants to know what it would take to deploy this for MULTIPLE users on AWS (production-grade), including: how the project works internally today, what must change in the code, which AWS services to use, and how to migrate the data.

## Your task: produce a comprehensive report covering

### 1. Current internal architecture (read the code)
- Routes/handlers and how rendering works (templates, partials, OOB swaps).
- Storage model: meta.json layout, image pipeline (original → .display 1200px → .thumb 400px), lazy generation, content-type-by-bytes.
- Concurrency model (imgMu) and why it's single-instance-safe only.
- Docker setup (named volume, healthcheck), env/config approach.
- Explicit list of limitations for multi-user production: no auth, no per-user isolation, local FS, single instance, no backups of metadata, no rate limiting, etc.

### 2. AWS migration plan (multi-user, production)
For each concern, recommend the AWS service with a one-line rationale:
- Compute: ECS Fargate vs AWS App Runner vs Lightsail (recommend one for a small Go app; justify).
- Container registry: ECR.
- Object storage for images: S3 (+ lifecycle rules, versioning on/off).
- CDN: CloudFront (static assets + images + app).
- Metadata storage: RDS (PostgreSQL) vs DynamoDB — recommend ONE with rationale (this app stores name/description/created_at per plant; images referenced by key).
- Secrets: Secrets Manager or SSM Parameter Store.
- Auth: Amazon Cognito (hosted UI) vs simple JWT — recommend with rationale for a small multi-user app (family/friends scale).
- Networking: VPC, multi-AZ, ALB, Route 53, ACM for TLS.
- Monitoring: CloudWatch logs/metrics/alarms, X-Ray optional.
- CI/CD: GitHub Actions → ECR → ECS (or CodePipeline) — recommend one.
- WAF/rate limiting: optional layer.

### 3. Required code changes (be specific, file-level)
- Introduce a storage interface (local FS implementation today; S3 implementation for prod) — which functions to abstract (saveImages, createThumbnail, loadPlant, handleServeImage, totalImageSize...).
- Metadata: replace meta.json read/write with a DB layer (schema suggestion: users, plants, images tables or DynamoDB items).
- Per-user scoping: plants keyed by user (e.g. S3 key prefix <user-id>/<slug>/..., DB rows with user_id FK).
- Auth middleware for all mutating + read routes; how to integrate with HTMX (401 handling, hx-redirect), Alpine.
- Image pipeline on S3: generate display/thumb at upload (upload to S3 as objects) vs on-the-fly with caching — recommend one.
- Static assets served by CloudFront; hashed filenames for cache busting.
- Config via env vars (S3 bucket, DB DSN, region...), remove hardcoded paths.
- Multi-instance safety: remove reliance on process-local mutex; make lazy generation idempotent (S3 HEAD-then-put or pre-generate at upload).
- Graceful shutdown, /health endpoint for ALB, request timeouts.
- Upload streaming to S3 (io.Pipe / S3 multipart) instead of local temp files; size limits preserved.
- Footer total size: aggregate query instead of filesystem walk.

### 4. Data migration
- From the Docker volume: export meta.json files → DB seed; copy images → S3 (aws s3 sync or rclone); generate derived versions if missing.

### 5. Deployment architecture
- ASCII diagram of the target architecture (users → Route53/CloudFront → ALB → ECS Fargate (multi-AZ) → S3 + RDS; Cognito; CloudWatch).
- Phased rollout: Phase 0 (today), Phase 1 (MVP on AWS, single "admin" user), Phase 2 (auth + multi-user), Phase 3 (hardening: WAF, backups, autoscaling) — with rough effort/cost notes per phase.

### 6. Security checklist
- HTTPS everywhere, auth, per-user authorization, upload validation (types/size — already present), rate limiting, S3 bucket policy (private, only via app/CloudFront signed or OAI/OAC), DB in private subnet, secrets not in image, backup strategy.

Keep the report well-structured and concrete. Cite specific functions/files from the codebase where relevant.

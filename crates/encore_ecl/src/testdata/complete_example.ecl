
version 1

// Baseline service limits for all environments.
for service {
    cpu: >= 0.25 & <= 8 | default 0.5
    memory: >= 256Mi & <= 16Gi | default 512Mi
}

// Production services get safer defaults and tighter limits.
for service if env.type == "production" {
    cpu: >= 1 & <= 4 | default 1
    memory: >= 1Gi & <= 8Gi | default 1Gi
    instances.min: >= 1 | default 1
    instances.max: <= 20
}

// Payments production services get larger defaults.
for service if env.type == "production" && team == "payments" {
    cpu: default 2
    memory: default 2Gi
}

// The API service needs a larger production default.
service "api" if env.type == "production" && team == "payments" {
    cpu: default 3
}

// Cloud Run-specific production behavior.
for service if env.type == "production" && provider == "gcp" && implementation == "cloud_run" {
    provider.gcp.cloud_run.cpu_always_allocated: true
    provider.gcp.cloud_run.min_instances: >= 1 | default 1
}

// Buckets should not be public by default.
for bucket {
    public_access: false
    versioning: true
}

// Customer data buckets need retention.
for bucket if tags.data == "customer" {
    backup_retention: >= 30d | default 30d
}

// Production SQL databases require stronger data protection.
for sql_database if env.type == "production" {
    backup_retention: >= 30d | default 30d
    point_in_time_recovery: true
    deletion_protection: true
}

// Main production database gets longer retention.
sql_database "main" if env.type == "production" {
    backup_retention: >= 90d | default 90d
}

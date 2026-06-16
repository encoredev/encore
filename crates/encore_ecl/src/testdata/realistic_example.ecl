version 1
if env.type == "production" {
    for service {
        cpu: >= 1 & <= 4 | default 1
        memory: >= 1Gi & <= 8Gi | default 1Gi
        instances.min: >= 1 | default 1
    }
    sql_cluster "main" {
        engine: "postgres"
        version: "16"
        cpu: >= 2 & <= 16 | default 4
        memory: >= 8Gi & <= 64Gi | default 16Gi
        storage: >= 100Gi | default 100Gi
        backup_retention: >= 30d | default 30d
        point_in_time_recovery: true
        high_availability: true
    }
    sql_cluster "audit" {
        engine: "postgres"
        version: "16"
        cpu: >= 4 & <= 32 | default 8
        memory: >= 16Gi & <= 128Gi | default 32Gi
        storage: >= 500Gi | default 1Ti
        backup_retention: >= 90d | default 90d
    }
    for sql_database {
        cluster: default sql_cluster.main
    }
    sql_database "audit" {
        cluster: sql_cluster.audit & {
            backup_retention: >= 90d
        }
    }
    for sql_database if tags.data == "customer" {
        cluster: {
            backup_retention: >= 30d
            point_in_time_recovery: true
            high_availability: true
        }
    }
    for bucket {
        public_access: false
        versioning: true
    }
}

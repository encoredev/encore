---
seotitle: Pre-installed PostgreSQL extensions on Encore
seodesc: See the list of pre-installed PostgreSQL extensions available when using Encore
title: PostgreSQL Extensions
subtitle: The list
infobox: {
  title: "SQL Databases",
  import: "encore.dev/storage/sqldb"
}
---

Encore uses the [encoredotdev/postgres](https://github.com/encoredev/postgres-image) docker image for local development,
CI/CD, and for databases hosted on Encore Cloud.

The docker image ships with the following PostgreSQL extensions available for use (via `CREATE EXTENSION`):

| Extension                      | Version |
|--------------------------------|---------|
| address_standardizer           | 3.3.4   |
| adminpack                      | 2.1     |
| amcheck                        | 1.3     |
| autoinc                        | 1.0     |
| bloom                          | 1.0     |
| btree_gin                      | 1.3     |
| btree_gist                     | 1.7     |
| citext                         | 1.6     |
| cube                           | 1.5     |
| dblink                         | 1.2     |
| dict_int                       | 1.0     |
| dict_xsyn                      | 1.0     |
| earthdistance                  | 1.1     |
| file_fdw                       | 1.0     |
| fuzzystrmatch                  | 1.1     |
| hstore                         | 1.8     |
| insert_username                | 1.0     |
| intagg                         | 1.1     |
| intarray                       | 1.5     |
| isn                            | 1.2     |
| lo                             | 1.1     |
| ltree                          | 1.2     |
| moddatetime                    | 1.0     |
| old_snapshot                   | 1.0     |
| pageinspect                    | 1.11    |
| pg_buffercache                 | 1.3     |
| pg_freespacemap                | 1.2     |
| pg_prewarm                     | 1.2     |
| pg_stat_statements             | 1.10    |
| pg_surgery                     | 1.0     |
| pg_trgm                        | 1.6     |
| pg_visibility                  | 1.2     |
| pg_walinspect                  | 1.0     |
| pgcrypto                       | 1.3     |
| pgrowlocks                     | 1.2     |
| pgstattuple                    | 1.5     |
| plpgsql                        | 1.0     |
| postgis                        | 3.3.4   |
| postgis-3                      | 3.3.4   |
| postgis_raster                 | 3.3.4   |
| postgis_raster-3               | 3.3.4   |
| postgis_sfcgal                 | 3.3.4   |
| postgis_sfcgal-3               | 3.3.4   |
| postgis_tiger_geocoder         | 3.3.4   |
| postgis_tiger_geocoder-3       | 3.3.4   |
| postgis_topology               | 3.3.4   |
| postgis_topology-3             | 3.3.4   |
| postgres_fdw                   | 1.1     |
| refint                         | 1.0     |
| seg                            | 1.4     |
| sslinfo                        | 1.2     |
| tablefunc                      | 1.0     |
| tcn                            | 1.0     |
| tsm_system_rows                | 1.0     |
| tsm_system_time                | 1.0     |
| unaccent                       | 1.1     |
| uuid-ossp                      | 1.1     |
| vector                         | 0.4.4   |
| xml2                           | 1.1     |

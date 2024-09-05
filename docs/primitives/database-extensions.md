---
seotitle: Pre-installed PostgreSQL extensions
seodesc: See the list of pre-installed PostgreSQL extensions available when using Encore
title: PostgreSQL Extensions
subtitle: Pre-installed extensions
infobox: {
  title: "SQL Databases",
  import: "encore.dev/storage/sqldb"
}
lang: go
---

Encore uses the [encoredotdev/postgres](https://github.com/encoredev/postgres-image) docker image for local development, CI/CD, and for databases hosted on Encore Cloud.

The docker image ships with the following PostgreSQL extensions pre-installed and available for use (via `CREATE EXTENSION`):

| Extension                    | Description                               |
| ---------------------------- | ----------------------------------------- |
| refint                       | Implements referential integrity          |
| pg_buffercache               | Examines shared buffer cache              |
| pg_freespacemap              | Examines free space map (FSM)             |
| plpgsql                      | PL/pgSQL procedural language              |
| citext                       | Case-insensitive character strings        |
| adminpack                    | Administrative functions                  |
| moddatetime                  | Tracks last modification time             |
| amcheck                      | Verifies relation integrity               |
| seg                          | Represents line segments or intervals     |
| pg_stat_statements           | Tracks SQL statement statistics           |
| pg_trgm                      | Text similarity and trigram-based search  |
| isn                          | International product numbering standards |
| btree_gist                   | Indexes common datatypes in GiST          |
| intarray                     | 1-D integer array operations              |
| pg_surgery                   | Repairs damaged relations                 |
| uuid-ossp                    | Generates UUIDs                           |
| insert_username              | Tracks table changes                      |
| bloom                        | Bloom filter index                        |
| pgcrypto                     | Cryptographic functions                   |
| dblink                       | Connects to other PostgreSQL databases    |
| tsm_system_rows              | Row-based table sampling                  |
| pg_prewarm                   | Preloads relation data                    |
| old_snapshot                 | Supports old_snapshot_threshold           |
| pageinspect                  | Low-level page inspection                 |
| intagg                       | Integer aggregation (obsolete)            |
| pg_visibility                | Examines visibility maps                  |
| cube                         | Multidimensional cube data type           |
| tablefunc                    | Table manipulation functions              |
| xml2                         | XPath and XSLT support                    |
| fuzzystrmatch                | String similarity functions               |
| pg_walinspect                | Inspects Write-Ahead Log                  |
| btree_gin                    | Indexes common datatypes in GIN           |
| sslinfo                      | SSL certificate information               |
| tcn                          | Triggered change notifications            |
| hstore                       | Key-value pair storage                    |
| dict_int                     | Integer text search dictionary            |
| earthdistance                | Great-circle distance calculations        |
| file_fdw                     | Flat file foreign data wrapper            |
| autoinc                      | Auto-incrementing fields                  |
| ltree                        | Hierarchical tree-like structures         |
| unaccent                     | Removes accents from text                 |
| pgrowlocks                   | Shows row-level locking info              |
| tsm_system_time              | Time-based table sampling                 |
| dict_xsyn                    | Extended synonym text search              |
| pgstattuple                  | Tuple-level statistics                    |
| postgres_fdw                 | PostgreSQL foreign data wrapper           |
| lo                           | Large Object maintenance                  |
| postgis_sfcgal               | PostGIS SFCGAL functions                  |
| address_standardizer_data_us | US address standardization data           |
| address_standardizer         | Parses addresses into elements            |
| postgis_topology             | PostGIS topology types and functions      |
| postgis                      | PostGIS geometry and geography types      |
| postgis_raster               | PostGIS raster functions                  |
| postgis_tiger_geocoder       | PostGIS TIGER geocoder                    |
| vector                       | Vector data type and indexing             |

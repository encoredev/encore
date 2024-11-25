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

| Extension                      | Version | Description                                                                                                         |
| ------------------------------ | ------- | ------------------------------------------------------------------------------------------------------------------- |
| refint                         | 1.0     | functions for implementing referential integrity (obsolete)                                                         |
| pg_buffercache                 | 1.3     | examine the shared buffer cache                                                                                     |
| pg_freespacemap                | 1.2     | examine the free space map (FSM)                                                                                    |
| plpgsql                        | 1.0     | PL/pgSQL procedural language                                                                                        |
| citext                         | 1.6     | data type for case-insensitive character strings                                                                    |
| adminpack                      | 2.1     | administrative functions for PostgreSQL                                                                             |
| moddatetime                    | 1.0     | functions for tracking last modification time                                                                       |
| amcheck                        | 1.3     | functions for verifying relation integrity                                                                          |
| seg                            | 1.4     | data type for representing line segments or floating-point intervals                                                |
| pg_stat_statements             | 1.10    | track planning and execution statistics of all SQL statements executed                                              |
| pg_trgm                        | 1.6     | text similarity measurement and index searching based on trigrams                                                   |
| isn                            | 1.2     | data types for international product numbering standards                                                            |
| btree_gist                     | 1.7     | support for indexing common datatypes in GiST                                                                       |
| intarray                       | 1.5     | functions, operators, and index support for 1-D arrays of integers                                                  |
| pg_surgery                     | 1.0     | extension to perform surgery on a damaged relation                                                                  |
| uuid-ossp                      | 1.1     | generate universally unique identifiers (UUIDs)                                                                     |
| insert_username                | 1.0     | functions for tracking who changed a table                                                                          |
| bloom                          | 1.0     | bloom access method - signature file based index                                                                    |
| pgcrypto                       | 1.3     | cryptographic functions                                                                                             |
| dblink                         | 1.2     | connect to other PostgreSQL databases from within a database                                                        |
| tsm_system_rows                | 1.0     | TABLESAMPLE method which accepts number of rows as a limit                                                          |
| pg_prewarm                     | 1.2     | prewarm relation data                                                                                               |
| old_snapshot                   | 1.0     | utilities in support of old_snapshot_threshold                                                                      |
| pageinspect                    | 1.11    | inspect the contents of database pages at a low level                                                               |
| intagg                         | 1.1     | integer aggregator and enumerator (obsolete)                                                                        |
| pg_visibility                  | 1.2     | examine the visibility map (VM) and page-level visibility info                                                      |
| cube                           | 1.5     | data type for multidimensional cubes                                                                                |
| tablefunc                      | 1.0     | functions that manipulate whole tables, including crosstab                                                          |
| xml2                           | 1.1     | XPath querying and XSLT                                                                                             |
| fuzzystrmatch                  | 1.1     | determine similarities and distance between strings                                                                 |
| pg_walinspect                  | 1.0     | functions to inspect contents of PostgreSQL Write-Ahead Log                                                         |
| btree_gin                      | 1.3     | support for indexing common datatypes in GIN                                                                        |
| sslinfo                        | 1.2     | information about SSL certificates                                                                                  |
| tcn                            | 1.0     | Triggered change notifications                                                                                      |
| hstore                         | 1.8     | data type for storing sets of (key, value) pairs                                                                    |
| dict_int                       | 1.0     | text search dictionary template for integers                                                                        |
| earthdistance                  | 1.1     | calculate great-circle distances on the surface of the Earth                                                        |
| file_fdw                       | 1.0     | foreign-data wrapper for flat file access                                                                           |
| autoinc                        | 1.0     | functions for autoincrementing fields                                                                               |
| ltree                          | 1.2     | data type for hierarchical tree-like structures                                                                     |
| unaccent                       | 1.1     | text search dictionary that removes accents                                                                         |
| pgrowlocks                     | 1.2     | show row-level locking information                                                                                  |
| tsm_system_time                | 1.0     | TABLESAMPLE method which accepts time in milliseconds as a limit                                                    |
| dict_xsyn                      | 1.0     | text search dictionary template for extended synonym processing                                                     |
| pgstattuple                    | 1.5     | show tuple-level statistics                                                                                         |
| postgres_fdw                   | 1.1     | foreign-data wrapper for remote PostgreSQL servers                                                                  |
| lo                             | 1.1     | Large Object maintenance                                                                                            |
| postgis_sfcgal-3               | 3.4.2   | PostGIS SFCGAL functions                                                                                            |
| address_standardizer_data_us-3 | 3.4.2   | Address Standardizer US dataset example                                                                             |
| address_standardizer-3         | 3.4.2   | Used to parse an address into constituent elements. Generally used to support geocoding address normalization step. |
| postgis_topology-3             | 3.4.2   | PostGIS topology spatial types and functions                                                                        |
| postgis-3                      | 3.4.2   | PostGIS geometry and geography spatial types and functions                                                          |
| postgis_raster-3               | 3.4.2   | PostGIS raster types and functions                                                                                  |
| postgis_tiger_geocoder-3       | 3.4.2   | PostGIS tiger geocoder and reverse geocoder                                                                         |
| vector                         | 0.7.0   | vector data type and ivfflat and hnsw access methods                                                                |
| postgis                        | 3.4.2   | PostGIS geometry and geography spatial types and functions                                                          |
| address_standardizer           | 3.4.2   | Used to parse an address into constituent elements. Generally used to support geocoding address normalization step. |
| postgis_topology               | 3.4.2   | PostGIS topology spatial types and functions                                                                        |
| postgis_tiger_geocoder         | 3.4.2   | PostGIS tiger geocoder and reverse geocoder                                                                         |
| address_standardizer_data_us   | 3.4.2   | Address Standardizer US dataset example                                                                             |
| postgis_sfcgal                 | 3.4.2   | PostGIS SFCGAL functions                                                                                            |
| postgis_raster                 | 3.4.2   | PostGIS raster types and functions                                                                                  |

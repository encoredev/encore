# pgproxy

PGProxy is a proxy for the Postgres wire protocol that allows for
customizing authentication and backend selection by overriding
the startup message flow.

Once authenticated, PGProxy falls back to being a dumb proxy
that simple shuffles bytes back and forth.

Internally pgproxy vendors simplified versions of the excellent
[pgproto3](https://github.com/jackc/pgproto3) and [pgio](https://github.com/jackc/pgio).
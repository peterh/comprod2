Comprod2
========

This is a fork of [comprod](https://github.com/peterh/comprod), with
backwards-incompatible changes.

The storage engine changes from gob to sqlite. This brings the advantage of
ACID storage, and allows the administrator to modify game state without having
to restart the comprod instance.

The hash function changes from sha1 to argon2 (for passwords) or SHAKE128 KMAC
(for other uses of hashes).

I do not recommend running this instead of comprod. This fork was primarily
created as an excuse to play with SQL.

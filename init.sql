DROP TABLE IF EXISTS url CASCADE;
DROP TABLE IF EXISTS user_info CASCADE;
DROP TABLE IF EXISTS request CASCADE;

CREATE TABLE IF NOT EXISTS user_info
(
    id       SERIAL PRIMARY KEY,
    username TEXT,
    password TEXT
);

CREATE TABLE IF NOT EXISTS url
(
    id          SERIAL PRIMARY KEY,
    scheme      TEXT,
    opaque      TEXT,
    "user"      INTEGER REFERENCES user_info (id),
    host        TEXT,
    path        TEXT,
    raw_path    TEXT,
    force_query bool,
    raw_query   TEXT,
    fragment    TEXT
);

CREATE TABLE IF NOT EXISTS request
(
    id             SERIAL PRIMARY KEY,
    url_id         INTEGER REFERENCES url (id),
    proto          TEXT,
    proto_major    INTEGER,
    proto_minor    INTEGER,
-- Header Header
    body           bytea,
    content_length INTEGER,
    Host           TEXT,
-- Form url.Values
-- MultipartForm *multipart.Form
    remote_addr    TEXT,
    request_uri    TEXT
);
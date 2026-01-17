CREATE TABLE IF NOT EXISTS bin (
	id		VARCHAR(64) NOT NULL PRIMARY KEY,
	readonly	BOOLEAN NOT NULL,
	updated_at	TIMESTAMP NOT NULL,
	created_at	TIMESTAMP NOT NULL,
	expired_at	TIMESTAMP NOT NULL,
	deleted_at	TIMESTAMP,
	approved_at	TIMESTAMP,
	downloads	BIGINT NOT NULL,
	updates		BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS file_content (
	sha256		VARCHAR(128) NOT NULL PRIMARY KEY,
	bytes		BIGINT NOT NULL,
	md5		VARCHAR(128) NOT NULL,
	mime		VARCHAR(128) NOT NULL,
	in_storage	BOOLEAN NOT NULL DEFAULT false,
	blocked		BOOLEAN NOT NULL DEFAULT false,
	created_at	TIMESTAMP NOT NULL,
	last_referenced_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS file (
	id		BIGSERIAL NOT NULL PRIMARY KEY,
	bin_id		VARCHAR(64) REFERENCES bin(id) ON DELETE CASCADE,
	filename        VARCHAR(128) NOT NULL,
	sha256		VARCHAR(128) NOT NULL REFERENCES file_content(sha256) ON DELETE RESTRICT,
	downloads	BIGINT NOT NULL,
	updates 	BIGINT NOT NULL,
	ip		VARCHAR(128) NOT NULL,
	headers		TEXT NOT NULL,
	updated_at	TIMESTAMP NOT NULL,
	created_at	TIMESTAMP NOT NULL,
	deleted_at	TIMESTAMP,
	UNIQUE(bin_id, filename)
);

CREATE TABLE IF NOT EXISTS transaction (
	id		BIGSERIAL NOT NULL PRIMARY KEY,
	bin_id		VARCHAR(64) NOT NULL,
	operation	TEXT NOT NULL,
	timestamp	TIMESTAMP NOT NULL,
	completed	TIMESTAMP NOT NULL,
	ip		VARCHAR(128) NOT NULL,
	method		VARCHAR(128) NOT NULL,
	path		TEXT NOT NULL,
	filename	TEXT,
	headers		TEXT NOT NULL,
	status		INT NOT NULL,
	req_bytes	BIGINT NOT NULL,
	resp_bytes	BIGINT NOT NULL,
	UNIQUE(id)
);

CREATE TABLE IF NOT EXISTS client (
	ip					VARCHAR(64) NOT NULL PRIMARY KEY,
	asn					INT NOT NULL,
	asn_organization			VARCHAR(128) NOT NULL DEFAULT '',
	network					VARCHAR(64) NOT NULL,
	city					VARCHAR(64) NOT NULL,
	country					VARCHAR(64) NOT NULL,
	continent				VARCHAR(64) NOT NULL,
	proxy					BOOLEAN NOT NULL,
	requests				BIGINT NOT NULL,
	first_active_at				TIMESTAMP NOT NULL,
	last_active_at				TIMESTAMP NOT NULL,
	banned_at				TIMESTAMP,
	banned_by				VARCHAR(64) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bin_id ON transaction(bin_id);
CREATE INDEX IF NOT EXISTS idx_ip ON transaction(ip);
CREATE INDEX IF NOT EXISTS idx_transaction_timestamp ON transaction(timestamp);
CREATE INDEX IF NOT EXISTS idx_file_deleted_at ON file(deleted_at);
CREATE INDEX IF NOT EXISTS idx_bin_deleted_at_expired_at ON bin(expired_at, deleted_at);
CREATE INDEX IF NOT EXISTS idx_sha256 ON file(sha256);
CREATE INDEX IF NOT EXISTS idx_client_banned_at ON client(banned_at, last_active_at);
CREATE INDEX IF NOT EXISTS idx_client_ip_active ON client(ip, last_active_at);
CREATE INDEX IF NOT EXISTS idx_file_content_in_storage ON file_content(in_storage);

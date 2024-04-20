CREATE TABLE bin (
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

CREATE TABLE file (
	id		BIGSERIAL NOT NULL PRIMARY KEY,
	bin_id		VARCHAR(64) REFERENCES bin(id) ON DELETE CASCADE,
	filename        VARCHAR(128) NOT NULL,
	in_storage	BOOLEAN NOT NULL,
	mime		VARCHAR(128) NOT NULL,
	bytes		BIGINT NOT NULL,
	md5		VARCHAR(128) NOT NULL,
	sha256		VARCHAR(128) NOT NULL,
	downloads	BIGINT NOT NULL,
	updates 	BIGINT NOT NULL,
	ip		VARCHAR(128) NOT NULL,
	client_id	VARCHAR(128) NOT NULL,
	headers		TEXT NOT NULL,
	updated_at	TIMESTAMP NOT NULL,
	created_at	TIMESTAMP NOT NULL,
	deleted_at	TIMESTAMP,
	UNIQUE(bin_id, filename)
);

CREATE TABLE transaction (
	id		BIGSERIAL NOT NULL PRIMARY KEY,
	bin_id		VARCHAR(64) NOT NULL,
	operation	TEXT NOT NULL,
	timestamp	TIMESTAMP NOT NULL,
	completed	TIMESTAMP NOT NULL,
	ip		VARCHAR(128) NOT NULL,
	client_id	VARCHAR(128) NOT NULL,
	method		VARCHAR(128) NOT NULL,
	path		TEXT NOT NULL,
	filename	TEXT,
	headers		TEXT NOT NULL,
	status		INT NOT NULL,
	req_bytes	BIGINT NOT NULL,
	resp_bytes	BIGINT NOT NULL,
	UNIQUE(id)
);

CREATE TABLE autonomous_system (
	asn					INT NOT NULL PRIMARY KEY,
	organization				VARCHAR(128) NOT NULL,
	requests				BIGINT NOT NULL,
	first_active_at				TIMESTAMP NOT NULL,
	last_active_at				TIMESTAMP NOT NULL,
	banned_at				TIMESTAMP
);

CREATE TABLE client (
	ip					VARCHAR(64) NOT NULL PRIMARY KEY,
	asn					INT NOT NULL,
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

CREATE INDEX idx_bin_id ON transaction(bin_id);
CREATE INDEX idx_ip ON transaction(ip);
CREATE INDEX idx_cid ON transaction(client_id);
CREATE INDEX idx_transaction_timestamp ON transaction(timestamp);
CREATE INDEX idx_file_deleted_at_in_storage ON file(deleted_at, in_storage);
CREATE INDEX idx_bin_deleted_at_expired_at ON bin(expired_at, deleted_at);
CREATE INDEX idx_sha256 ON file(sha256);

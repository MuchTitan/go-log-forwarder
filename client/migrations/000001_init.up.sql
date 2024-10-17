CREATE TABLE IF NOT EXISTS tail_file_state (
    filepath TEXT PRIMARY KEY,
    last_send_line INTEGER,
    checksum BLOB,
    inode_number INTEGER
);

CREATE TABLE IF NOT EXISTS router(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    output TEXT NOT NULL,
    input TEXT NOT NULL,
    parser TEXT NOT NULL,
    filter TEXT
);

CREATE TABLE IF NOT EXISTS retry_data(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    data BLOB NOT NULL,
    outputs TEXT NOT NULL,
    status BOOLEAN DEFAULT 0,
    router_id INTEGER,
    FOREIGN KEY(router_id) REFERENCES router(id),
    UNIQUE(data, router_id)
);

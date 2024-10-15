CREATE TABLE IF NOT EXISTS tail_file_state (
    filepath TEXT PRIMARY KEY,
    last_send_line INTEGER,
    checksum BLOB,
    inode_number INTEGER
);

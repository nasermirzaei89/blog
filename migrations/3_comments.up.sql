CREATE TABLE
    comments (
        id TEXT NOT NULL PRIMARY KEY,
        post_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        content TEXT NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        FOREIGN KEY (post_id) REFERENCES posts (id),
        FOREIGN KEY (user_id) REFERENCES users (id)
    );
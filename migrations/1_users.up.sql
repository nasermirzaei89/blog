CREATE TABLE
    users (
        id TEXT NOT NULL PRIMARY KEY,
        username TEXT NOT NULL UNIQUE,
        email_address TEXT NOT NULL UNIQUE,
        password_hash TEXT NOT NULL,
        name TEXT NOT NULL,
        avatar_url TEXT NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
    );

INSERT INTO
    users (
        id,
        username,
        email_address,
        password_hash,
        name,
        avatar_url,
        created_at,
        updated_at
    )
VALUES
    (
        '9ebccc6b-a40b-4cdd-b0db-5781e14a47bb',
        'admin',
        'admin@localhost',
        '$2a$10$CwTycUXWue0Thq9StjUM0uJ8iKZp.Xx5CX63Hg.Z8MRG4E9z3a8lG',
        'Admin',
        '',
        '2025-01-01T14:30:00Z',
        '2025-01-01T14:30:00Z'
    );
CREATE TABLE settings
(
    uuid  VARCHAR PRIMARY KEY,
    name  VARCHAR NOT NULL UNIQUE,
    value VARCHAR NOT NULL
);

INSERT INTO settings
    (uuid, name, value)
VALUES ("5c229021-0ae0-49a0-a809-4f4c310552a6", "title", "AwesomePress"),
       ("ef53493d-1b10-4783-a298-623458bd1e5d", "tagline", "My awesome blog"),
       ("0b2bbf7a-7d5e-42e6-885e-2cbe61ac9d7a", "timeZone", "Local");

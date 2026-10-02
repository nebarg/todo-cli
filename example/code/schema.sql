CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL -- todo1 make this unique
);

-- TODO: drop this once the import has moved to users
CREATE TABLE legacy_users (
    id INTEGER PRIMARY KEY,
    name TEXT
);

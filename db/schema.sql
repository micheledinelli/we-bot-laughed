CREATE TABLE chapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_number INTEGER NOT NULL,
    url TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
    chat_id INTEGER PRIMARY KEY,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_chapters_chapter_number ON chapters(chapter_number DESC);

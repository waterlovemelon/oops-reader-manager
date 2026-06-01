-- Add word_count to catalog_books and a composite index for status+title queries.
-- Date: 2026-06-01
-- Note: catalog_books was created by backend 005_init_catalog_books.sql and
-- extended by manager 002_extend_catalog_books_for_manager.sql.

ALTER TABLE `catalog_books`
    ADD COLUMN `word_count` BIGINT UNSIGNED NULL AFTER `chapter_count`;

CREATE INDEX `idx_catalog_books_status_title` ON `catalog_books` (`status`, `title`);

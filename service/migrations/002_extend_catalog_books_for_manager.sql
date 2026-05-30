ALTER TABLE `catalog_books`
    ADD COLUMN `description` TEXT NULL AFTER `author`,
    ADD COLUMN `format` VARCHAR(32) NOT NULL DEFAULT 'epub' AFTER `description`,
    ADD COLUMN `cover_storage_path` VARCHAR(1024) NULL AFTER `storage_path`,
    ADD COLUMN `source` VARCHAR(64) NOT NULL DEFAULT 'batch_import' AFTER `status`,
    ADD COLUMN `uploaded_at` DATETIME NULL AFTER `indexed_at`,
    ADD COLUMN `published_at` DATETIME NULL AFTER `uploaded_at`,
    ADD COLUMN `deleted_at` DATETIME NULL AFTER `published_at`,
    ADD COLUMN `updated_by` VARCHAR(191) NULL AFTER `deleted_at`;

ALTER TABLE `catalog_books`
    MODIFY COLUMN `status` VARCHAR(32) NOT NULL DEFAULT 'active';

CREATE INDEX `idx_catalog_books_format_status` ON `catalog_books` (`format`, `status`);
CREATE INDEX `idx_catalog_books_uploaded_at` ON `catalog_books` (`uploaded_at`);

-- Asset names are creation-time facts. Historical rows remain unknown.
ALTER TABLE composite_outputs
 ADD COLUMN name VARCHAR(255) NULL,
 ADD COLUMN file_name VARCHAR(255) NULL;

ALTER TABLE dashboard_layout
    ADD COLUMN IF NOT EXISTS excluded_host_group_ids TEXT[] NOT NULL DEFAULT '{}';

-- CHG-20261105-115: annotate the existing executable template versions.
-- Only task editability metadata changes; parameter types and execution stay
-- unchanged. Unknown template codes/versions/executors remain untouched.
UPDATE compose_templates
SET schema_json = JSON_SET(schema_json,
    '$.properties.start_seconds.editable', CAST('true' AS JSON),
    '$.properties.end_seconds.editable', CAST('true' AS JSON))
WHERE code = 'basic_cut' AND version = 1 AND executor = 'basic_cut_v1';

UPDATE compose_templates
SET schema_json = JSON_SET(schema_json,
    '$.properties.guide_video_url.editable', CAST('false' AS JSON),
    '$.properties.insert_seconds.editable', CAST('true' AS JSON),
    '$.properties.guide_duration_seconds.editable', CAST('true' AS JSON),
    '$.properties.max_duration_seconds.editable', CAST('true' AS JSON))
WHERE version = 1 AND (
    (code = 'guide_insert' AND executor = 'guide_insert_v1') OR
    (code = 'guide_pip' AND executor = 'guide_pip_v1'));

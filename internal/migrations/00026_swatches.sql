-- +goose Up
-- LDS v0.0.6 ships seven swatch tokens: teal and orange are gone. Stored
-- colours move to the nearest remaining swatch (teal → green, orange →
-- coral). Events keep the colour they recorded.
UPDATE projects SET colour = 'green' WHERE colour = 'teal';
UPDATE projects SET colour = 'coral' WHERE colour = 'orange';
ALTER TABLE projects DROP CONSTRAINT projects_colour;
ALTER TABLE projects ADD CONSTRAINT projects_colour
    CHECK (colour IN ('blue', 'green', 'purple', 'pink', 'coral', 'yellow', 'grey'));

UPDATE issue_types SET colour = CASE colour WHEN 'teal' THEN 'green' ELSE 'coral' END WHERE colour IN ('teal', 'orange');
UPDATE status_categories SET colour = CASE colour WHEN 'teal' THEN 'green' ELSE 'coral' END WHERE colour IN ('teal', 'orange');
UPDATE labels SET colour = CASE colour WHEN 'teal' THEN 'green' ELSE 'coral' END WHERE colour IN ('teal', 'orange');

-- Select options: {"options": [{"id", "name", "colour"}, …], …}.
UPDATE custom_fields f SET options = jsonb_set(f.options, '{options}', (
    SELECT jsonb_agg(CASE o->>'colour'
        WHEN 'teal' THEN jsonb_set(o, '{colour}', '"green"')
        WHEN 'orange' THEN jsonb_set(o, '{colour}', '"coral"')
        ELSE o END ORDER BY i)
    FROM jsonb_array_elements(f.options->'options') WITH ORDINALITY AS t(o, i)))
WHERE jsonb_typeof(f.options->'options') = 'array'
  AND jsonb_path_exists(f.options, '$.options[*] ? (@.colour == "teal" || @.colour == "orange")');

-- +goose Down
-- The remapped colours stay: teal and orange are allowed again but not restored.
ALTER TABLE projects DROP CONSTRAINT projects_colour;
ALTER TABLE projects ADD CONSTRAINT projects_colour
    CHECK (colour IN ('blue', 'green', 'teal', 'purple', 'pink', 'coral', 'orange', 'yellow', 'grey'));

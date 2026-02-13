-- Remove candidates for unwatched_tv_low rules before deleting the rules
DELETE FROM maintenance_candidates WHERE rule_id IN (
    SELECT id FROM maintenance_rules WHERE criterion_type = 'unwatched_tv_low'
);
DELETE FROM maintenance_exclusions WHERE rule_id IN (
    SELECT id FROM maintenance_rules WHERE criterion_type = 'unwatched_tv_low'
);
DELETE FROM maintenance_rules WHERE criterion_type = 'unwatched_tv_low';

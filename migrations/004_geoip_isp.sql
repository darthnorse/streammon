-- Add ISP field to ip_geo_cache for ASN/organization data
ALTER TABLE ip_geo_cache ADD COLUMN isp TEXT DEFAULT '';

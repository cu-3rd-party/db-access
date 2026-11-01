-- The layout of pg-grm for local runs; in production it comes from homelab postgres/cluster-grm.yaml.
CREATE ROLE cu_owner LOGIN PASSWORD 'owner';
CREATE ROLE cu_provisioner LOGIN CREATEROLE PASSWORD 'provisioner' IN ROLE cu_owner;
CREATE DATABASE "cu-sql" OWNER cu_owner;
CREATE DATABASE "cu-db" OWNER cu_owner;

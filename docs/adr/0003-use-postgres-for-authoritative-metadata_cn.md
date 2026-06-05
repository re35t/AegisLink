# ADR 0003：使用 PostgreSQL 存储权威元数据

使用 PostgreSQL 存储 users、agents、organizations、capabilities 和 audit metadata，因为这些记录需要事务一致性。

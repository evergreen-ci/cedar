Presto SQL DDL Log
==================

Executed in March 2022:

```sql
-- DROP TABLE awsdatacatalog.dev_prod_live.cedar_test_results_landing;
CREATE TABLE awsdatacatalog.dev_prod_live.cedar_test_results_landing (
  version                 VARCHAR,  
  variant                 VARCHAR,
  task_id                 VARCHAR,
  task_name               VARCHAR,
  request_type            VARCHAR,
  execution               INTEGER,
  created_at              TIMESTAMP,
  results ARRAY(
    ROW(test_name           VARCHAR,
        display_test_name   VARCHAR,
        group_id            VARCHAR,
        trial               INTEGER,
        status              VARCHAR,
            -- status is either "pass" or "fail"
        log_test_name       VARCHAR,
        log_url             VARCHAR,
        raw_log_url         VARCHAR,
        line_num            INTEGER,
        task_create_time    TIMESTAMP,
        test_start_time     TIMESTAMP,
        test_end_time       TIMESTAMP)
  ),
  task_create_iso         VARCHAR,
  project                 VARCHAR
) WITH (
    format = 'Parquet',
    partitioned_by = ARRAY['task_create_iso', 'project']
);
```


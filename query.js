cur = db.perf_results.find({
    "info.project": "sys-perf-4.2",
    "info.variant": "linux-3-shard",
    "info.task_name": "mongos_large_catalog_workloads",
    "info.test_name": "canary_client-cpuloop-10x",
    "$expr": {
        "$lt": [
            "$analysis.processed_at",
            "$rollups.processed_at",
        ],
    },
})

while (cur.hasNext()) {
    printjson(cur.next())
}

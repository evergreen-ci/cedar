cur = db.perf_results.aggregate([
    {
        "$match": {
            "info.project": "performance-4.4",
            "info.variant": "linux-wt-standalone",
            "info.task_name": "update",
            "info.test_name": "Update.MatchedElementWithinArray",
            "info.order": {"$exists": true},
            "info.mainline": true,
        },
    },
    {
        "$unwind": "$rollups.stats",
    },
    {
        "$group": {
            "_id": {
                "project": "$info.project",
                "variant": "$info.variant",
                "task": "$info.task_name",
                "test": "$info.test_name",
                "measurement": "$rollups.stats.name",
                "args": "$info.args",
            },
            "time_series": {
                "$push": {
    		"value": {
                        "$ifNull": [
    			"$rollups.stats.val",
    		        0,
                        ],
    	         },
    		"order": "$info.order",
    		"perf_result_id": "$_id",
    		"version": "$info.version",
                "execution": "$info.execution",
                },
            },
        },
     },
    {
        "$project": {"_id": 1, "count": {"$size": "$time_series"}},
    },
])

while (cur.hasNext()) {
    printjson(cur.next())
}

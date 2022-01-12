/*
cur = db.perf_results.aggregate([
    {
        "$match": {
            "info.order": {"$exists": true},
            "info.mainline": true,
            "info.project": {"$exists": true},
            "info.variant": {"$exists": true},
            "info.task_name": {"$exists": true},
            "info.test_name": {"$exists": true},
            "rollups.stats": {"$not": {"$size": 0}},
         },
    },
    {
        "$match": {
            "$expr": {
                "$lt": [
                    "$analysis.processed_at",
                    "$rollups.processed_at",
                ],
            },
        },
    },
    {
        "$group": {
            "_id": {
                "project": "$info.project",
                "variant": "$info.variant",
                "task": "$info.task_name",
                "test": "$info.test_name",
            },
        },
    },
    {
        "$replaceRoot": {
            "newRoot": "$_id",
        },
    },
])

cur = db.perf_results.aggregate([
      {
        "$match": {
          "info.project": {
            "$exists": true
          },
          "info.variant": {
            "$exists": true
          },
          "info.task_name": {
            "$exists": true
          },
          "info.test_name": {
            "$exists": true
          },
          "rollups.stats": {
            "$not": {
              "$size": 0
            }
          },
          "info.order": {
            "$exists": true
          },
          "info.mainline": true
        }
      },
      {
        "$match": {
          "$expr": {
            "$lt": [
              "$analysis.processed_at",
              "$rollups.processed_at"
            ]
          }
        }
      },
      {
        "$group": {
          "_id": {
            "variant": "$info.variant",
            "task": "$info.task_name",
            "test": "$info.test_name",
            "project": "$info.project"
          }
        }
      },
      {
        "$replaceRoot": {
          "newRoot": "$_id"
        }
      },
])

while (cur.hasNext()) {
    printjson(cur.next())
}
*/

cur = db.perf_results.find({
      "info.project": {
        "$exists": true
      },
      "info.variant": {
        "$exists": true
      },
      "info.task_name": {
        "$exists": true
      },
      "info.test_name": {
        "$exists": true
      },
      "rollups.stats": {
        "$not": {
          "$size": 0
        }
      },
      "info.order": {
        "$exists": true
      },
      "info.mainline": true,
      "created_at": {"$lte": ISODate("2021-06-28T00:00:00.000Z")},
      "$and": [ {"info.args": {"$not": {"$size": 0}}}, {"info.args": {"$not": {"$size": 1}}} ],
}).limit(100000)

while (cur.hasNext()) {
    printjson(cur.next())
}

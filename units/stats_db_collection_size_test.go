package units

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/mgo.v2/bson"
)

// to test some of the syntax in stats_db_collection_size.go
func TestMakeStatsCmd(t *testing.T) {
	assert := assert.New(t)
	db := cedar.GetEnvironment().GetDB()
	ctx := context.Background()
	_, err := db.ListCollectionNames(ctx, primitive.D{})
	assert.NoError(err)
	collName := "foo"
	statsCmd := primitive.D{{Key: "collStats", Value: collName}}
	var statsResult bson.M
	err = db.RunCommand(ctx, statsCmd).Decode(&statsResult)
	assert.NoError(err)
}

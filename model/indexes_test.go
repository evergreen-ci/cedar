package model

import (
	"context"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestCheckIndexes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for testName, testCase := range map[string]struct {
		required  []SystemIndexes
		present   []SystemIndexes
		expectErr bool
	}{
		"SucceedsWithAllRequiredInDB": {
			required: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test",
				},
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: -1}},
					Collection: "index_test",
				},
			},
			present: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test",
				},
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: -1}},
					Collection: "index_test",
				},
			},
			expectErr: false,
		},
		"FailsWithRequiredMissingFromDB": {
			required: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test",
				},
			},
			expectErr: true,
		},
		"SucceedsWithExtraInDB": {
			required: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test",
				},
			},
			present: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test",
				},
				{
					Keys:       bson.D{{Key: "foo", Value: 1}},
					Collection: "index_test",
				},
			},
			expectErr: false,
		},
		"SucceedsWithAllRequiredInDBWithMultipleCollections": {
			required: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test1",
				},
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: -1}},
					Collection: "index_test2",
				},
			},
			present: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test1",
				},
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: -1}},
					Collection: "index_test2",
				},
			},
			expectErr: false,
		},
		"FailsWithRequiredInWrongCollection": {
			required: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test1",
				},
			},
			present: []SystemIndexes{
				{
					Keys:       bson.D{{Key: "foo", Value: 1}, {Key: "bar", Value: 1}},
					Collection: "index_test2",
				},
			},
			expectErr: true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			env := cedar.GetEnvironment()
			db := env.GetDB()
			require.NoError(t, db.Drop(ctx))
			defer func() {
				assert.NoError(t, db.Drop(ctx))
			}()

			for _, index := range testCase.present {
				_, err := db.Collection(index.Collection).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: index.Keys})
				require.NoError(t, err)
			}

			err := CheckIndexes(ctx, db, testCase.required)
			if testCase.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

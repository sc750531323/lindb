package service

import (
	"testing"

	"gopkg.in/check.v1"

	"github.com/eleme/lindb/mock"
	"github.com/eleme/lindb/models"
	"github.com/eleme/lindb/pkg/state"
)

type testDatabaseSRVSuite struct {
	mock.RepoTestSuite
}

func TestDatabaseSRV(t *testing.T) {
	check.Suite(&testDatabaseSRVSuite{})
	check.TestingT(t)
}

func (ts *testDatabaseSRVSuite) TestDatabase(c *check.C) {
	repo, _ := state.NewRepo(state.Config{
		Endpoints: ts.Cluster.Endpoints,
	})

	db := NewDatabaseService(repo)
	database := models.Database{
		Name:          "test",
		NumOfShard:    12,
		ReplicaFactor: 3,
	}
	err := db.Save(database)
	if err != nil {
		c.Fatal(err)
	}
	err = db.Save(models.Database{
		NumOfShard:    12,
		ReplicaFactor: 3,
	})
	c.Assert(err, check.NotNil)
	err = db.Save(models.Database{
		Name:          "test",
		ReplicaFactor: 3,
	})
	c.Assert(err, check.NotNil)
	err = db.Save(models.Database{
		Name:       "test",
		NumOfShard: 12,
	})
	c.Assert(err, check.NotNil)

	database2, _ := db.Get("test")
	c.Assert(database, check.DeepEquals, database2)
}

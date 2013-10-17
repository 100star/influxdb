package datastore

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"os"
	"parser"
	"protocol"
	"testing"
	"time"
)

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) {
	TestingT(t)
}

type DatastoreSuite struct{}

var _ = Suite(&DatastoreSuite{})

const DB_DIR = "/tmp/chronosdb/datastore_test"

func newDatastore(c *C) Datastore {
	os.MkdirAll(DB_DIR, 0744)
	db, err := NewLevelDbDatastore(DB_DIR)
	c.Assert(err, Equals, nil)
	return db
}

func cleanup(db Datastore) {
	if db != nil {
		db.Close()
	}
	os.RemoveAll(DB_DIR)
}

func stringToSeries(seriesString string, timestamp int64, c *C) *protocol.Series {
	series := &protocol.Series{}
	err := json.Unmarshal([]byte(seriesString), &series)
	c.Assert(err, IsNil)
	for _, point := range series.Points {
		point.Timestamp = &timestamp
	}
	return series
}

func executeQuery(database, query string, db Datastore, c *C) *protocol.Series {
	q, errQ := parser.ParseQuery(query)
	c.Assert(errQ, IsNil)
	done := make(chan int, 1)
	resultSeries := &protocol.Series{}
	yield := func(series *protocol.Series) error {
		resultSeries = series
		done <- 1
		return nil
	}
	err := db.ExecuteQuery(database, q, yield)
	c.Assert(err, IsNil)
	<-done
	return resultSeries
}

func (self *DatastoreSuite) TestCanWriteAndRetrievePoints(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)
	mock := `
  {
    "points": [
      {
        "values": [
          {
            "int_value": 3
          }
        ],
        "sequence_number": 1
      },
      {
        "values": [
          {
            "int_value": 2
          }
        ],
        "sequence_number": 2
      }
    ],
    "name": "foo",
    "fields": [
      {
        "type": "INT32",
        "name": "value"
      }
    ]
  }`
	pointTime := time.Now().Unix()
	series := stringToSeries(mock, pointTime, c)
	err := db.WriteSeriesData("test", series)
	c.Assert(err, IsNil)
	q, errQ := parser.ParseQuery("select value from foo;")
	c.Assert(errQ, IsNil)
	done := make(chan int, 1)
	resultSeries := &protocol.Series{}
	yield := func(series *protocol.Series) error {
		resultSeries = series
		done <- 1
		return nil
	}
	err = db.ExecuteQuery("test", q, yield)
	c.Assert(err, IsNil)
	<-done
	c.Assert(resultSeries, Not(IsNil))
	c.Assert(len(resultSeries.Points), Equals, 2)
	c.Assert(len(resultSeries.Fields), Equals, 1)
	c.Assert(*resultSeries.Points[0].SequenceNumber, Equals, uint32(2))
	c.Assert(*resultSeries.Points[1].SequenceNumber, Equals, uint32(1))
	c.Assert(*resultSeries.Points[0].Timestamp, Equals, pointTime)
	c.Assert(*resultSeries.Points[1].Timestamp, Equals, pointTime)
	c.Assert(*resultSeries.Points[0].Values[0].IntValue, Equals, int32(2))
	c.Assert(*resultSeries.Points[1].Values[0].IntValue, Equals, int32(3))
	c.Assert(resultSeries, Not(DeepEquals), series)
}

func (self *DatastoreSuite) TestCanPersistDataAndWriteNewData(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	mock := `
  {
    "points": [
      {
        "values": [
          {
            "int_value": 3
          }
        ],
        "sequence_number": 1
      }
    ],
    "name": "foo",
    "fields": [
      {
        "type": "INT32",
        "name": "value"
      }
    ]
  }`
	series := stringToSeries(mock, time.Now().Unix(), c)
	err := db.WriteSeriesData("asdf", series)
	c.Assert(err, IsNil)
	results := executeQuery("asdf", "select value from foo;", db, c)
	c.Assert(results, DeepEquals, series)
	db.Close()
	db = newDatastore(c)
	defer cleanup(db)
	results = executeQuery("asdf", "select value from foo;", db, c)
	c.Assert(results, DeepEquals, series)
}

func (self *DatastoreSuite) TestCanWriteDataWithDifferentTimesAndSeries(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)
	mock := `{
    "points":[{"values":[{"double_value":23.2}],"sequence_number":3}],
    "name": "events",
    "fields": [{"type": "DOUBLE", "name": "blah"}]}`
	secondAgo := time.Now().Add(-time.Second).Unix()
	eventsSeries := stringToSeries(mock, secondAgo, c)
	err := db.WriteSeriesData("db1", eventsSeries)
	c.Assert(err, IsNil)
	mock = `{
    "points":[{"values":[{"int_value":4}],"sequence_number":3}],
    "name": "foo",
    "fields": [{"type": "INT32", "name": "val"}]}`
	fooSeries := stringToSeries(mock, secondAgo, c)
	err = db.WriteSeriesData("db1", fooSeries)
	c.Assert(err, IsNil)

	results := executeQuery("db1", "select blah from events;", db, c)
	c.Assert(results, DeepEquals, eventsSeries)
	results = executeQuery("db1", "select val from foo;", db, c)
	c.Assert(results, DeepEquals, fooSeries)

	now := time.Now().Unix()
	mock = `{
    "points":[{"values":[{"double_value": 0.1}],"sequence_number":1}],
    "name":"events",
    "fields": [{"type": "DOUBLE", "name": "blah"}]}`

	newEvents := stringToSeries(mock, now, c)
	err = db.WriteSeriesData("db1", newEvents)
	c.Assert(err, IsNil)

	results = executeQuery("db1", "select blah from events;", db, c)
	c.Assert(len(results.Points), Equals, 2)
	c.Assert(len(results.Fields), Equals, 1)
	c.Assert(*results.Points[0].SequenceNumber, Equals, uint32(1))
	c.Assert(*results.Points[1].SequenceNumber, Equals, uint32(3))
	c.Assert(*results.Points[0].Timestamp, Equals, now)
	c.Assert(*results.Points[1].Timestamp, Equals, secondAgo)
	c.Assert(*results.Points[0].Values[0].DoubleValue, Equals, float64(0.1))
	c.Assert(*results.Points[1].Values[0].DoubleValue, Equals, float64(23.2))
	results = executeQuery("db1", "select val from foo;", db, c)
	c.Assert(results, DeepEquals, fooSeries)
}

func (self *DatastoreSuite) TestCanWriteDataToDifferentDatabases(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)
	mock := `{
    "points":[{"values":[{"double_value":23.2}],"sequence_number":3}],
    "name": "events",
    "fields": [{"type": "DOUBLE", "name": "blah"}]}`
	secondAgo := time.Now().Add(-time.Second).Unix()
	db1Series := stringToSeries(mock, secondAgo, c)
	err := db.WriteSeriesData("db1", db1Series)
	c.Assert(err, IsNil)
	mock = `{
    "points":[{"values":[{"double_value":3.2}],"sequence_number":2}],
    "name": "events",
    "fields": [{"type": "DOUBLE", "name": "blah"}]}`
	otherDbSeries := stringToSeries(mock, secondAgo, c)
	err = db.WriteSeriesData("other_db", otherDbSeries)
	c.Assert(err, IsNil)

	results := executeQuery("db1", "select blah from events;", db, c)
	c.Assert(results, DeepEquals, db1Series)
	results = executeQuery("other_db", "select blah from events;", db, c)
	c.Assert(results, DeepEquals, otherDbSeries)
}

func (self *DatastoreSuite) TestCanQueryBasedOnTime(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)

	minutesAgo := time.Now().Add(-10 * time.Minute).Unix()
	now := time.Now().Unix()
	mock := `{
    "points":[{"values":[{"int_value":4}],"sequence_number":3}],
    "name": "foo",
    "fields": [{"type": "INT32", "name": "val"}]}`
	oldData := stringToSeries(mock, minutesAgo, c)
	err := db.WriteSeriesData("db1", oldData)
	c.Assert(err, IsNil)

	mock = `{
    "points":[{"values":[{"int_value":3}],"sequence_number":3}],
    "name": "foo",
    "fields": [{"type": "INT32", "name": "val"}]}`
	newData := stringToSeries(mock, now, c)
	err = db.WriteSeriesData("db1", newData)
	c.Assert(err, IsNil)

	results := executeQuery("db1", "select val from foo where time>now()-1m;", db, c)
	c.Assert(results, DeepEquals, newData)
	results = executeQuery("db1", "select val from foo where time>now()-1h and time<now()-1m;", db, c)
	c.Assert(results, DeepEquals, oldData)

	results = executeQuery("db1", "select val from foo;", db, c)
	c.Assert(len(results.Points), Equals, 2)
	c.Assert(len(results.Fields), Equals, 1)
	c.Assert(*results.Points[0].SequenceNumber, Equals, uint32(3))
	c.Assert(*results.Points[1].SequenceNumber, Equals, uint32(3))
	c.Assert(*results.Points[0].Timestamp, Equals, now)
	c.Assert(*results.Points[1].Timestamp, Equals, minutesAgo)
	c.Assert(*results.Points[0].Values[0].IntValue, Equals, int32(3))
	c.Assert(*results.Points[1].Values[0].IntValue, Equals, int32(4))
}

func (self *DatastoreSuite) TestCanDoWhereQueryEquals(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)

	mock := `{
    "points":[{"values":[{"string_value":"paul"}],"sequence_number":2},{"values":[{"string_value":"todd"}],"sequence_number":1}],
    "name":"events",
    "fields":[{"type":"STRING","name":"name"}]
    }`
	allData := stringToSeries(mock, time.Now().Unix(), c)
	err := db.WriteSeriesData("db1", allData)
	c.Assert(err, IsNil)

	results := executeQuery("db1", "select name from events;", db, c)
	c.Assert(results, DeepEquals, allData)
	results = executeQuery("db1", "select name from events where name == 'paul';", db, c)
	c.Assert(len(results.Points), Equals, 1)
	c.Assert(len(results.Fields), Equals, 1)
	c.Assert(*results.Points[0].SequenceNumber, Equals, uint32(2))
	c.Assert(*results.Points[0].Values[0].StringValue, Equals, "paul")
}

func (self *DatastoreSuite) TestCanDoSelectStarQueries(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)

	mock := `{
    "points":[
      {"values":[{"int_value":3},{"string_value":"paul"}],"sequence_number":2},
      {"values":[{"int_value":1},{"string_value":"todd"}],"sequence_number":1}],
      "name":"user_things",
      "fields":[{"type":"INT32","name":"count"},{"type":"STRING","name":"name"}]
    }`
	series := stringToSeries(mock, time.Now().Unix(), c)
	err := db.WriteSeriesData("foobar", series)
	c.Assert(err, IsNil)
	results := executeQuery("foobar", "select * from user_things;", db, c)
	c.Assert(results, DeepEquals, series)
}

func (self *DatastoreSuite) TestCanDoCountStarQueries(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)

	mock := `{
    "points":[
      {"values":[{"int_value":3},{"string_value":"paul"}],"sequence_number":2},
      {"values":[{"int_value":1},{"string_value":"todd"}],"sequence_number":1}],
      "name":"user_things",
      "fields":[{"type":"INT32","name":"count"},{"type":"STRING","name":"name"}]
    }`
	series := stringToSeries(mock, time.Now().Unix(), c)
	err := db.WriteSeriesData("foobar", series)
	c.Assert(err, IsNil)
	results := executeQuery("foobar", "select count(*) from user_things;", db, c)
	c.Assert(len(results.Points), Equals, 2)
	c.Assert(len(results.Fields), Equals, 1)
	c.Assert(*results.Points[0].SequenceNumber, Equals, uint32(2))
	c.Assert(*results.Points[0].Values[0].IntValue, Equals, int32(3))
	c.Assert(*results.Points[1].SequenceNumber, Equals, uint32(1))
	c.Assert(*results.Points[1].Values[0].IntValue, Equals, int32(1))
}

func (self *DatastoreSuite) TestLimitsPointsReturnedBasedOnQuery(c *C) {
	cleanup(nil)
	db := newDatastore(c)
	defer cleanup(db)

	mock := `{
    "points":[
      {"values":[{"int_value":3},{"string_value":"paul"}],"sequence_number":2},
      {"values":[{"int_value":1},{"string_value":"todd"}],"sequence_number":1}],
      "name":"user_things",
      "fields":[{"type":"INT32","name":"count"},{"type":"STRING","name":"name"}]
    }`
	series := stringToSeries(mock, time.Now().Unix(), c)
	err := db.WriteSeriesData("foobar", series)
	c.Assert(err, IsNil)
	results := executeQuery("foobar", "select name from user_things limit 1;", db, c)
	c.Assert(len(results.Points), Equals, 1)
	c.Assert(len(results.Fields), Equals, 1)
	c.Assert(*results.Points[0].SequenceNumber, Equals, uint32(2))
	c.Assert(*results.Points[0].Values[0].StringValue, Equals, "paul")
}
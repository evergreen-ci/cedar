package rest

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/evergreen-ci/gimlet"
	"github.com/stretchr/testify/suite"
)

type PerfHandlerSuite struct {
	sc data.MockConnector
	rh map[string]gimlet.RouteHandler

	suite.Suite
}

func (s *PerfHandlerSuite) setup() {
	s.sc = data.MockConnector{
		CachedPerformanceResults: map[string]model.APIPerformanceResult{
			"abc": model.APIPerformanceResult{
				Name: model.ToAPIString("abc"),
				Info: model.APIPerformanceResultInfo{
					Version:  model.ToAPIString("1"),
					TaskID:   model.ToAPIString("123"),
					TaskName: model.ToAPIString("taskname0"),
					Tags:     []string{"a", "b"},
				},
			},
			"def": model.APIPerformanceResult{
				Name:        model.ToAPIString("def"),
				CreatedAt:   model.NewTime(time.Date(2018, time.December, 1, 1, 1, 0, 0, time.UTC)),
				CompletedAt: model.NewTime(time.Date(2018, time.December, 1, 2, 1, 0, 0, time.UTC)),
				Info: model.APIPerformanceResultInfo{
					Version:  model.ToAPIString("1"),
					TaskID:   model.ToAPIString("123"),
					TaskName: model.ToAPIString("taskname0"),
					Tags:     []string{"a"},
				},
			},
			"ghi": model.APIPerformanceResult{
				Name:        model.ToAPIString("ghi"),
				CreatedAt:   model.NewTime(time.Date(2018, time.December, 1, 1, 1, 0, 0, time.UTC)),
				CompletedAt: model.NewTime(time.Date(2018, time.December, 1, 2, 1, 0, 0, time.UTC)),
				Info: model.APIPerformanceResultInfo{
					Version:  model.ToAPIString("1"),
					TaskID:   model.ToAPIString("123"),
					TaskName: model.ToAPIString("taskname0"),
					Tags:     []string{"b"},
				},
			},

			"jkl": model.APIPerformanceResult{
				Name:        model.ToAPIString("jkl"),
				CreatedAt:   model.NewTime(time.Date(2018, time.December, 1, 1, 1, 0, 0, time.UTC)),
				CompletedAt: model.NewTime(time.Date(2018, time.December, 1, 2, 1, 0, 0, time.UTC)),
				Info: model.APIPerformanceResultInfo{
					Version:  model.ToAPIString("1"),
					TaskID:   model.ToAPIString("123"),
					TaskName: model.ToAPIString("taskname0"),
					Tags:     []string{"a", "b", "c"},
				},
			},
			"lmn": model.APIPerformanceResult{
				Name:        model.ToAPIString("lmn"),
				CreatedAt:   model.NewTime(time.Date(2018, time.December, 5, 1, 1, 0, 0, time.UTC)),
				CompletedAt: model.NewTime(time.Date(2018, time.December, 6, 2, 1, 0, 0, time.UTC)),
				Info: model.APIPerformanceResultInfo{
					Version:  model.ToAPIString("2"),
					TaskID:   model.ToAPIString("456"),
					TaskName: model.ToAPIString("taskname1"),
				},
			},
		},
	}
	s.sc.ChildMap = map[string][]string{
		"abc": []string{"def"},
		"def": []string{"jkl"},
	}
	s.rh = map[string]gimlet.RouteHandler{
		"id":        makeGetPerfById(&s.sc),
		"task_id":   makeGetPerfByTaskId(&s.sc),
		"task_name": makeGetPerfByTaskName(&s.sc),
		"version":   makeGetPerfByVersion(&s.sc),
		"children":  makeGetPerfChildren(&s.sc),
	}
}

func TestPerfHandlerSuite(t *testing.T) {
	s := new(PerfHandlerSuite)
	s.setup()
	suite.Run(t, s)
}

func (s *PerfHandlerSuite) TestPerfGetByIdHandlerFound() {
	rh := s.rh["id"]
	rh.(*perfGetByIdHandler).id = "abc"
	expected := s.sc.CachedPerformanceResults["abc"]

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(&expected, resp.Data())
}

func (s *PerfHandlerSuite) TestPerfGetByIdHandlerNotFound() {
	rh := s.rh["id"]
	rh.(*perfGetByIdHandler).id = "DNE"

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *PerfHandlerSuite) TestPerfGetByTaskIdHandlerFound() {
	rh := s.rh["task_id"]
	rh.(*perfGetByTaskIdHandler).taskId = "123"
	rh.(*perfGetByTaskIdHandler).interval = util.TimeRange{
		StartAt: time.Date(2018, time.November, 5, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Now(),
	}
	rh.(*perfGetByTaskIdHandler).tags = []string{"a", "b"}
	expected := []model.APIPerformanceResult{s.sc.CachedPerformanceResults["jkl"]}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *PerfHandlerSuite) TestPerfGetByTaskIdHandlerNotFound() {
	rh := s.rh["task_id"]
	rh.(*perfGetByTaskIdHandler).taskId = "555"
	rh.(*perfGetByTaskIdHandler).interval = util.TimeRange{
		StartAt: time.Date(2018, time.November, 5, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *PerfHandlerSuite) TestPerfGetByTaskNameHandlerFound() {
	rh := s.rh["task_name"]
	rh.(*perfGetByTaskNameHandler).taskName = "taskname0"
	rh.(*perfGetByTaskNameHandler).interval = util.TimeRange{
		StartAt: time.Date(2018, time.November, 5, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Now(),
	}
	rh.(*perfGetByTaskNameHandler).tags = []string{"a", "b"}
	expected := []model.APIPerformanceResult{s.sc.CachedPerformanceResults["jkl"]}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *PerfHandlerSuite) TestPerfGetByTaskNameHandlerNotFound() {
	rh := s.rh["task_name"]
	rh.(*perfGetByTaskNameHandler).taskName = "taskname2"
	rh.(*perfGetByTaskNameHandler).interval = util.TimeRange{
		StartAt: time.Date(2018, time.November, 5, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *PerfHandlerSuite) TestPerfGetByVersionHandlerFound() {
	rh := s.rh["version"]
	rh.(*perfGetByVersionHandler).version = "1"
	rh.(*perfGetByVersionHandler).interval = util.TimeRange{
		StartAt: time.Date(2018, time.November, 5, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Now(),
	}
	rh.(*perfGetByVersionHandler).tags = []string{"a", "b"}
	expected := []model.APIPerformanceResult{s.sc.CachedPerformanceResults["jkl"]}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *PerfHandlerSuite) TestPerfGetByVersionHandlerNotFound() {
	rh := s.rh["version"]
	rh.(*perfGetByVersionHandler).version = "2"
	rh.(*perfGetByVersionHandler).interval = util.TimeRange{
		StartAt: time.Date(2018, time.November, 5, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Now(),
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *PerfHandlerSuite) TestPerfGetChildrenHandlerFound() {
	rh := s.rh["children"]
	rh.(*perfGetChildrenHandler).id = "abc"
	rh.(*perfGetChildrenHandler).maxDepth = 1
	rh.(*perfGetChildrenHandler).tags = []string{"a"}
	expected := []model.APIPerformanceResult{
		s.sc.CachedPerformanceResults["abc"],
		s.sc.CachedPerformanceResults["def"],
	}

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.Equal(http.StatusOK, resp.Status())
	s.Require().NotNil(resp.Data())
	s.Equal(expected, resp.Data())
}

func (s *PerfHandlerSuite) TestPerfGetChildrenHandlerNotFound() {
	rh := s.rh["children"]
	rh.(*perfGetChildrenHandler).id = "DNE"
	rh.(*perfGetChildrenHandler).maxDepth = 5

	resp := rh.Run(context.TODO())
	s.Require().NotNil(resp)
	s.NotEqual(http.StatusOK, resp.Status())
}

func (s *PerfHandlerSuite) TestParse() {
	for _, test := range []struct {
		urlString string
		handler   string
	}{
		{
			handler:   "task_id",
			urlString: "http://example.com/perf/task_id/task_id0",
		},
		{
			handler:   "task_name",
			urlString: "http://example.com/perf/task_name/task_name0",
		},
		{
			handler:   "version",
			urlString: "http://example.com/perf/version/verison0",
		},
	} {
		s.testParseValid(test.handler, test.urlString)
		s.testParseInvalid(test.handler, test.urlString)
		s.testParseDefaults(test.handler, test.urlString)
	}
}

func (s *PerfHandlerSuite) testParseValid(handler, urlString string) {
	ctx := context.Background()
	urlString += "?started_after=2012-11-01T22:08:00%2B00:00"
	urlString += "&finished_before=2013-11-01T22:08:00%2B00:00"
	urlString += "&tags=hello&tags=world"
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	expectedInterval := util.TimeRange{
		StartAt: time.Date(2012, time.November, 1, 22, 8, 0, 0, time.UTC),
		EndAt:   time.Date(2013, time.November, 1, 22, 8, 0, 0, time.UTC),
	}
	expectedTags := []string{"hello", "world"}
	rh := s.rh[handler]

	err := rh.Parse(ctx, req)
	s.Equal(expectedInterval, getInterval(rh, handler))
	s.Equal(expectedTags, getTags(rh, handler))
	s.NoError(err)
}

func (s *PerfHandlerSuite) testParseInvalid(handler, urlString string) {
	ctx := context.Background()
	invalidStart := "?started_after=hello"
	invalidEnd := "?finished_before=world"
	req := &http.Request{Method: "GET"}
	rh := s.rh[handler]

	req.URL, _ = url.Parse(urlString + invalidStart)
	err := rh.Parse(ctx, req)
	s.Error(err)

	req.URL, _ = url.Parse(urlString + invalidEnd)
	err = rh.Parse(ctx, req)
	s.Error(err)
}

func (s *PerfHandlerSuite) testParseDefaults(handler, urlString string) {
	ctx := context.Background()
	req := &http.Request{Method: "GET"}
	req.URL, _ = url.Parse(urlString)
	lessThanTime := time.Now()
	// sleep to combat window's low time resolution
	time.Sleep(time.Second)
	rh := s.rh[handler]

	err := rh.Parse(ctx, req)
	interval := getInterval(rh, handler)
	s.Equal(time.Time{}, interval.StartAt)
	// ensure default EndAt time is within the time period in which the function
	// has been called
	s.True(interval.EndAt.After(lessThanTime))
	// sleep to combat window's low time resolution
	time.Sleep(time.Second)
	s.True(interval.EndAt.Before(time.Now()))
	s.Nil(getTags(rh, handler))
	s.NoError(err)
}

func getInterval(rh gimlet.RouteHandler, handler string) util.TimeRange {
	switch handler {
	case "task_id":
		return rh.(*perfGetByTaskIdHandler).interval
	case "task_name":
		return rh.(*perfGetByTaskNameHandler).interval
	case "version":
		return rh.(*perfGetByVersionHandler).interval
	default:
		return util.TimeRange{}
	}
}

func getTags(rh gimlet.RouteHandler, handler string) []string {
	switch handler {
	case "task_id":
		return rh.(*perfGetByTaskIdHandler).tags
	case "task_name":
		return rh.(*perfGetByTaskNameHandler).tags
	case "version":
		return rh.(*perfGetByVersionHandler).tags
	default:
		return []string{}
	}
}

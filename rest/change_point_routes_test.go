package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/gimlet"
	"github.com/stretchr/testify/suite"
)

type ChangePointSuite struct {
	rh map[string]gimlet.RouteHandler
	sc data.MockConnector

	suite.Suite
}

func (s *ChangePointSuite) setupCpSuite() {
	s.rh = map[string]gimlet.RouteHandler{
		"change_points_by_version": makeGetChangePointsByVersion(&s.sc),
	}
}

func TestChangePointSuite(t *testing.T) {
	s := new(ChangePointSuite)
	s.setupCpSuite()
	suite.Run(t, s)
}

func (s *ChangePointSuite) TestGetVersionWithChangePointsRouteParsing() {
	rh := s.rh["change_points_by_version"].(*perfGetChangePointsByVersionHandler)
	url := "/perf/project/some_project/change_points_by_version"
	url += "?page=0&pageSize=10"
	url += "&variantRegex=some_variant"
	url += "&versionRegex=some_version"
	url += "&taskRegex=some_task"
	url += "&testRegex=some_test"
	url += "&measurementRegex=some_measurement"
	url += "&threadLevel=1,2,3"
	req, err := http.NewRequest("GET", url, nil)
	s.Require().NoError(err)
	s.Require().NoError(rh.Parse(context.TODO(), req))
	s.Require().Equal(rh.args.Page, 0)
	s.Require().Equal(rh.args.PageSize, 10)
	s.Require().Equal(rh.args.VariantRegex, "some_variant")
	s.Require().Equal(rh.args.VersionRegex, "some_version")
	s.Require().Equal(rh.args.TaskRegex, "some_task")
	s.Require().Equal(rh.args.TestRegex, "some_test")
	s.Require().Equal(rh.args.MeasurementRegex, "some_measurement")
	arguments := map[string][]int{
		"thread_level": {1, 2, 3},
	}
	s.Require().Equal(rh.args.Arguments, arguments)
}

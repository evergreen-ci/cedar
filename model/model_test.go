package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commonModel interface {
	Setup(e cedar.Environment)
	IsNil() bool
	Find() error
	Save() error
}

type commonModelFactory func() commonModel

func TestModelInterface(t *testing.T) {
	models := []commonModelFactory{
		func() commonModel { return &CedarConfig{} },
		func() commonModel { return &CostConfig{} },
		func() commonModel { return &CostReport{} },
		func() commonModel { return &CostReportSummary{} },
		func() commonModel { return &GraphMetadata{} },
		func() commonModel { return &GraphEdge{} },
		func() commonModel { return &LogRecord{} },
		func() commonModel { return &Event{} },
		func() commonModel { return &SystemInformationRecord{} },
		func() commonModel { return &PerformanceResult{} },
	}
	oddballs := []interface{}{
		&LogSegment{},
	}
	slices := []interface{}{
		&DependencyGraphs{},
		&CostReports{},
	}
	assert.NotNil(t, oddballs)
	assert.NotNil(t, slices)

	env := cedar.GetEnvironment()

	conf, session, err := cedar.GetSessionWithConfig(env)
	require.NoError(t, err)
	require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())

	testCases := map[string]func(*testing.T, commonModel){
		"VerifyNillCheckIsCorrect": func(t *testing.T, m commonModel) {
			assert.True(t, m.IsNil())
		},
		"VerifySetEnv": func(t *testing.T, m commonModel) {
			assert.NotPanics(t, func() {
				m.Setup(cedar.GetEnvironment())
			})
		},
		"FindErrorsWithoutEnv": func(t *testing.T, m commonModel) {
			err := m.Find()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "env is nil")
		},
		"FindErrorsForMissingDocument": func(t *testing.T, m commonModel) {
			m.Setup(cedar.GetEnvironment())
			err := m.Find()
			assert.Error(t, err, "%+v", m)
		},
		"SaveErrorsForEmpty": func(t *testing.T, m commonModel) {
			m.Setup(cedar.GetEnvironment())
			assert.Error(t, m.Save())
		},
	}

	for _, factory := range models {
		modelName := strings.TrimLeft(fmt.Sprintf("%T", factory()), "*model.")
		t.Run(modelName, func(t *testing.T) {
			for name, test := range testCases {
				t.Run(name, func(t *testing.T) {
					test(t, factory())
				})
			}
		})
	}

	require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())
}

type commonModelSlice interface {
	Setup(e cedar.Environment)
	IsNil() bool
	Size() int
}

type checkSliceType func(commonModelSlice) error
type commonModelSliceFactory func() commonModelSlice

func TestCommonModelSlice(t *testing.T) {
	env := cedar.GetEnvironment()

	conf, session, err := cedar.GetSessionWithConfig(env)
	require.NoError(t, err)
	require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())

	defer func() {
		require.NoError(t, session.DB(conf.DatabaseName).DropDatabase())
	}()

	for _, helpers := range []struct {
		name     string
		factory  commonModelSliceFactory
		check    checkSliceType
		populate func(int, commonModelSlice)
	}{
		{
			name:    "CostReports",
			factory: func() commonModelSlice { return &CostReports{} },
			check: func(s commonModelSlice) error {
				type slicer interface {
					commonModelSlice
					Slice() []CostReport
				}
				sl, ok := s.(slicer)
				if !ok {
					return errors.New("incorrect type")
				} else if len(sl.Slice()) != sl.Size() {
					return errors.New("unexpected slice value")
				}

				return nil
			},
			populate: func(size int, m commonModelSlice) {
				sl := m.(*CostReports)
				for i := 0; i < size; i++ {
					sl.reports = append(sl.reports, createTestStruct())
				}
				sl.populated = true
			},
		},
		{
			name:    "CostReportSummaries",
			factory: func() commonModelSlice { return &CostReportSummaries{} },
			check: func(s commonModelSlice) error {
				type slicer interface {
					commonModelSlice
					Slice() []CostReportSummary
				}

				sl, ok := s.(slicer)
				if !ok {
					return errors.New("incorrect type")
				} else if len(sl.Slice()) != sl.Size() {
					return errors.New("unexpected slice value")
				}
				return nil
			},
			populate: func(size int, m commonModelSlice) {
				sl := m.(*CostReportSummaries)
				for i := 0; i < size; i++ {
					sl.reports = append(sl.reports, CostReportSummary{ID: fmt.Sprintf("test_doc_%d", i)})
				}
				sl.populated = true
			},
		},
		{
			name:    "DependencyGraphs",
			factory: func() commonModelSlice { return &DependencyGraphs{} },
			check: func(s commonModelSlice) error {
				type slicer interface {
					commonModelSlice
					Slice() []GraphMetadata
				}

				sl, ok := s.(slicer)
				if !ok {
					return errors.New("incorrect type")
				} else if len(sl.Slice()) != sl.Size() {
					return errors.New("unexpected slice value")
				}
				return nil
			},
			populate: func(size int, m commonModelSlice) {
				sl := m.(*DependencyGraphs)
				for i := 0; i < size; i++ {
					sl.graphs = append(sl.graphs, GraphMetadata{BuildID: fmt.Sprintln("grapid", i)})
				}
				sl.populated = true
			},
		},
		{
			name:    "SystemInfoRecords",
			factory: func() commonModelSlice { return &SystemInformationRecords{} },
			check: func(s commonModelSlice) error {
				type slicer interface {
					commonModelSlice
					Slice() []*SystemInformationRecord
				}

				sl, ok := s.(slicer)
				if !ok {
					return errors.New("incorrect type")
				} else if len(sl.Slice()) != sl.Size() {
					return errors.New("unexpected slice value")
				}
				return nil
			},
			populate: func(size int, m commonModelSlice) {
				sl := m.(*SystemInformationRecords)
				for i := 0; i < size; i++ {
					sl.slice = append(sl.slice, &SystemInformationRecord{ID: fmt.Sprintln("infor", i)})
				}
				sl.populated = true
			},
		},
		{
			name:    "LogSegments",
			factory: func() commonModelSlice { return &LogSegments{} },
			check: func(s commonModelSlice) error {
				type slicer interface {
					commonModelSlice
					Slice() []LogSegment
				}

				sl, ok := s.(slicer)
				if !ok {
					return errors.New("incorrect type")
				} else if len(sl.Slice()) != sl.Size() {
					return errors.New("unexpected slice value")
				}
				return nil
			},
			populate: func(size int, m commonModelSlice) {
				sl := m.(*LogSegments)
				for i := 0; i < size; i++ {
					sl.logs = append(sl.logs, LogSegment{ID: fmt.Sprintln("infor", i)})
				}
				sl.populated = true
			},
		},
		{
			name:    "Events",
			factory: func() commonModelSlice { return &Events{} },
			check: func(s commonModelSlice) error {
				type slicer interface {
					commonModelSlice
					Slice() []*Event
				}

				sl, ok := s.(slicer)
				if !ok {
					return errors.New("incorrect type")
				} else if len(sl.Slice()) != sl.Size() {
					return errors.New("unexpected slice value")
				}
				return nil
			},
			populate: func(size int, m commonModelSlice) {
				sl := m.(*Events)
				for i := 0; i < size; i++ {
					sl.slice = append(sl.slice, &Event{ID: fmt.Sprintln("event", i)})
				}
				sl.populated = true
			},
		},
	} {
		t.Run(helpers.name, func(t *testing.T) {
			require.NotNil(t, helpers.factory)
			require.NotNil(t, helpers.check)
			require.NotNil(t, helpers.populate)

			for _, test := range []struct {
				name  string
				check func(*testing.T, commonModelSlice)
			}{
				{
					name: "ValidateFixture",
					check: func(t *testing.T, m commonModelSlice) {
						assert.NotNil(t, m)
					},
				},
				{
					name: "SetupIsSave",
					check: func(t *testing.T, m commonModelSlice) {
						assert.NotPanics(t, func() { m.Setup(env) })
					},
				},
				{
					name: "ValidateSliceType",
					check: func(t *testing.T, m commonModelSlice) {
						assert.NoError(t, helpers.check(m))
					},
				},
				{
					name: "CheckIsNilByDefault",
					check: func(t *testing.T, m commonModelSlice) {
						assert.True(t, m.IsNil())
					},
				},
				{
					name: "PopulateCheck",
					check: func(t *testing.T, m commonModelSlice) {
						helpers.populate(10, m)
						assert.NoError(t, helpers.check(m))
						assert.Equal(t, 10, m.Size())
						assert.False(t, m.IsNil())
					},
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					test.check(t, helpers.factory())
				})
			}
		})
	}
}

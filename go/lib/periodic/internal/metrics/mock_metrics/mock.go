// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/scionproto/scion/go/lib/periodic/internal/metrics (interfaces: ExportMetric)

// Package mock_metrics is a generated GoMock package.
package mock_metrics

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	time "time"
)

// MockExportMetric is a mock of ExportMetric interface
type MockExportMetric struct {
	ctrl     *gomock.Controller
	recorder *MockExportMetricMockRecorder
}

// MockExportMetricMockRecorder is the mock recorder for MockExportMetric
type MockExportMetricMockRecorder struct {
	mock *MockExportMetric
}

// NewMockExportMetric creates a new mock instance
func NewMockExportMetric(ctrl *gomock.Controller) *MockExportMetric {
	mock := &MockExportMetric{ctrl: ctrl}
	mock.recorder = &MockExportMetricMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockExportMetric) EXPECT() *MockExportMetricMockRecorder {
	return m.recorder
}

// Event mocks base method
func (m *MockExportMetric) Event(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Event", arg0)
}

// Event indicates an expected call of Event
func (mr *MockExportMetricMockRecorder) Event(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Event", reflect.TypeOf((*MockExportMetric)(nil).Event), arg0)
}

// Period mocks base method
func (m *MockExportMetric) Period(arg0 time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Period", arg0)
}

// Period indicates an expected call of Period
func (mr *MockExportMetricMockRecorder) Period(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Period", reflect.TypeOf((*MockExportMetric)(nil).Period), arg0)
}

// Runtime mocks base method
func (m *MockExportMetric) Runtime(arg0 time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Runtime", arg0)
}

// Runtime indicates an expected call of Runtime
func (mr *MockExportMetricMockRecorder) Runtime(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Runtime", reflect.TypeOf((*MockExportMetric)(nil).Runtime), arg0)
}

// StartTimestamp mocks base method
func (m *MockExportMetric) StartTimestamp(arg0 time.Time) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StartTimestamp", arg0)
}

// StartTimestamp indicates an expected call of StartTimestamp
func (mr *MockExportMetricMockRecorder) StartTimestamp(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartTimestamp", reflect.TypeOf((*MockExportMetric)(nil).StartTimestamp), arg0)
}
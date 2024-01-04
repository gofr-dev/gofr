// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

// Package pubsub is a generated GoMock package.
package pubsub

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "gofr.dev/pkg/gofr/types"
)

// MockPublisherSubscriber is a mock of PublisherSubscriber interface.
type MockPublisherSubscriber struct {
	ctrl     *gomock.Controller
	recorder *MockPublisherSubscriberMockRecorder
}

// MockPublisherSubscriberMockRecorder is the mock recorder for MockPublisherSubscriber.
type MockPublisherSubscriberMockRecorder struct {
	mock *MockPublisherSubscriber
}

// NewMockPublisherSubscriber creates a new mock instance.
func NewMockPublisherSubscriber(ctrl *gomock.Controller) *MockPublisherSubscriber {
	mock := &MockPublisherSubscriber{ctrl: ctrl}
	mock.recorder = &MockPublisherSubscriberMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPublisherSubscriber) EXPECT() *MockPublisherSubscriberMockRecorder {
	return m.recorder
}

// Bind mocks base method.
func (m *MockPublisherSubscriber) Bind(message []byte, target interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Bind", message, target)
	ret0, _ := ret[0].(error)
	return ret0
}

// Bind indicates an expected call of Bind.
func (mr *MockPublisherSubscriberMockRecorder) Bind(message, target interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Bind", reflect.TypeOf((*MockPublisherSubscriber)(nil).Bind), message, target)
}

// CommitOffset mocks base method.
func (m *MockPublisherSubscriber) CommitOffset(offsets TopicPartition) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CommitOffset", offsets)
}

// CommitOffset indicates an expected call of CommitOffset.
func (mr *MockPublisherSubscriberMockRecorder) CommitOffset(offsets interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitOffset", reflect.TypeOf((*MockPublisherSubscriber)(nil).CommitOffset), offsets)
}

// HealthCheck mocks base method.
func (m *MockPublisherSubscriber) HealthCheck() types.Health {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck")
	ret0, _ := ret[0].(types.Health)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockPublisherSubscriberMockRecorder) HealthCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockPublisherSubscriber)(nil).HealthCheck))
}

// IsSet mocks base method.
func (m *MockPublisherSubscriber) IsSet() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSet")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsSet indicates an expected call of IsSet.
func (mr *MockPublisherSubscriberMockRecorder) IsSet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSet", reflect.TypeOf((*MockPublisherSubscriber)(nil).IsSet))
}

// Ping mocks base method.
func (m *MockPublisherSubscriber) Ping() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping")
	ret0, _ := ret[0].(error)
	return ret0
}

// Ping indicates an expected call of Ping.
func (mr *MockPublisherSubscriberMockRecorder) Ping() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*MockPublisherSubscriber)(nil).Ping))
}

// PublishEvent mocks base method.
func (m *MockPublisherSubscriber) PublishEvent(arg0 string, arg1 interface{}, arg2 map[string]string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishEvent", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishEvent indicates an expected call of PublishEvent.
func (mr *MockPublisherSubscriberMockRecorder) PublishEvent(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishEvent", reflect.TypeOf((*MockPublisherSubscriber)(nil).PublishEvent), arg0, arg1, arg2)
}

// PublishEventWithOptions mocks base method.
func (m *MockPublisherSubscriber) PublishEventWithOptions(key string, value interface{}, headers map[string]string, options *PublishOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishEventWithOptions", key, value, headers, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishEventWithOptions indicates an expected call of PublishEventWithOptions.
func (mr *MockPublisherSubscriberMockRecorder) PublishEventWithOptions(key, value, headers, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishEventWithOptions", reflect.TypeOf((*MockPublisherSubscriber)(nil).PublishEventWithOptions), key, value, headers, options)
}

// Subscribe mocks base method.
func (m *MockPublisherSubscriber) Subscribe() (*Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe")
	ret0, _ := ret[0].(*Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Subscribe indicates an expected call of Subscribe.
func (mr *MockPublisherSubscriberMockRecorder) Subscribe() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockPublisherSubscriber)(nil).Subscribe))
}

// SubscribeWithCommit mocks base method.
func (m *MockPublisherSubscriber) SubscribeWithCommit(arg0 CommitFunc) (*Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeWithCommit", arg0)
	ret0, _ := ret[0].(*Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeWithCommit indicates an expected call of SubscribeWithCommit.
func (mr *MockPublisherSubscriberMockRecorder) SubscribeWithCommit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeWithCommit", reflect.TypeOf((*MockPublisherSubscriber)(nil).SubscribeWithCommit), arg0)
}

// MockPublisherSubscriberV2 is a mock of PublisherSubscriberV2 interface.
type MockPublisherSubscriberV2 struct {
	ctrl     *gomock.Controller
	recorder *MockPublisherSubscriberV2MockRecorder
}

// MockPublisherSubscriberV2MockRecorder is the mock recorder for MockPublisherSubscriberV2.
type MockPublisherSubscriberV2MockRecorder struct {
	mock *MockPublisherSubscriberV2
}

// NewMockPublisherSubscriberV2 creates a new mock instance.
func NewMockPublisherSubscriberV2(ctrl *gomock.Controller) *MockPublisherSubscriberV2 {
	mock := &MockPublisherSubscriberV2{ctrl: ctrl}
	mock.recorder = &MockPublisherSubscriberV2MockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPublisherSubscriberV2) EXPECT() *MockPublisherSubscriberV2MockRecorder {
	return m.recorder
}

// Bind mocks base method.
func (m *MockPublisherSubscriberV2) Bind(message []byte, target interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Bind", message, target)
	ret0, _ := ret[0].(error)
	return ret0
}

// Bind indicates an expected call of Bind.
func (mr *MockPublisherSubscriberV2MockRecorder) Bind(message, target interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Bind", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).Bind), message, target)
}

// CommitOffset mocks base method.
func (m *MockPublisherSubscriberV2) CommitOffset(offsets TopicPartition) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CommitOffset", offsets)
}

// CommitOffset indicates an expected call of CommitOffset.
func (mr *MockPublisherSubscriberV2MockRecorder) CommitOffset(offsets interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitOffset", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).CommitOffset), offsets)
}

// HealthCheck mocks base method.
func (m *MockPublisherSubscriberV2) HealthCheck() types.Health {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck")
	ret0, _ := ret[0].(types.Health)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockPublisherSubscriberV2MockRecorder) HealthCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).HealthCheck))
}

// IsSet mocks base method.
func (m *MockPublisherSubscriberV2) IsSet() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSet")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsSet indicates an expected call of IsSet.
func (mr *MockPublisherSubscriberV2MockRecorder) IsSet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSet", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).IsSet))
}

// Pause mocks base method.
func (m *MockPublisherSubscriberV2) Pause() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Pause")
	ret0, _ := ret[0].(error)
	return ret0
}

// Pause indicates an expected call of Pause.
func (mr *MockPublisherSubscriberV2MockRecorder) Pause() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Pause", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).Pause))
}

// Ping mocks base method.
func (m *MockPublisherSubscriberV2) Ping() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping")
	ret0, _ := ret[0].(error)
	return ret0
}

// Ping indicates an expected call of Ping.
func (mr *MockPublisherSubscriberV2MockRecorder) Ping() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).Ping))
}

// PublishEvent mocks base method.
func (m *MockPublisherSubscriberV2) PublishEvent(arg0 string, arg1 interface{}, arg2 map[string]string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishEvent", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishEvent indicates an expected call of PublishEvent.
func (mr *MockPublisherSubscriberV2MockRecorder) PublishEvent(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishEvent", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).PublishEvent), arg0, arg1, arg2)
}

// PublishEventWithOptions mocks base method.
func (m *MockPublisherSubscriberV2) PublishEventWithOptions(key string, value interface{}, headers map[string]string, options *PublishOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishEventWithOptions", key, value, headers, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishEventWithOptions indicates an expected call of PublishEventWithOptions.
func (mr *MockPublisherSubscriberV2MockRecorder) PublishEventWithOptions(key, value, headers, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishEventWithOptions", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).PublishEventWithOptions), key, value, headers, options)
}

// Resume mocks base method.
func (m *MockPublisherSubscriberV2) Resume() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Resume")
	ret0, _ := ret[0].(error)
	return ret0
}

// Resume indicates an expected call of Resume.
func (mr *MockPublisherSubscriberV2MockRecorder) Resume() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Resume", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).Resume))
}

// Subscribe mocks base method.
func (m *MockPublisherSubscriberV2) Subscribe() (*Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe")
	ret0, _ := ret[0].(*Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Subscribe indicates an expected call of Subscribe.
func (mr *MockPublisherSubscriberV2MockRecorder) Subscribe() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).Subscribe))
}

// SubscribeWithCommit mocks base method.
func (m *MockPublisherSubscriberV2) SubscribeWithCommit(arg0 CommitFunc) (*Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeWithCommit", arg0)
	ret0, _ := ret[0].(*Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeWithCommit indicates an expected call of SubscribeWithCommit.
func (mr *MockPublisherSubscriberV2MockRecorder) SubscribeWithCommit(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeWithCommit", reflect.TypeOf((*MockPublisherSubscriberV2)(nil).SubscribeWithCommit), arg0)
}

// MockMQTTPublisherSubscriber is a mock of MQTTPublisherSubscriber interface.
type MockMQTTPublisherSubscriber struct {
	ctrl     *gomock.Controller
	recorder *MockMQTTPublisherSubscriberMockRecorder
}

// MockMQTTPublisherSubscriberMockRecorder is the mock recorder for MockMQTTPublisherSubscriber.
type MockMQTTPublisherSubscriberMockRecorder struct {
	mock *MockMQTTPublisherSubscriber
}

// NewMockMQTTPublisherSubscriber creates a new mock instance.
func NewMockMQTTPublisherSubscriber(ctrl *gomock.Controller) *MockMQTTPublisherSubscriber {
	mock := &MockMQTTPublisherSubscriber{ctrl: ctrl}
	mock.recorder = &MockMQTTPublisherSubscriberMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMQTTPublisherSubscriber) EXPECT() *MockMQTTPublisherSubscriberMockRecorder {
	return m.recorder
}

// Bind mocks base method.
func (m *MockMQTTPublisherSubscriber) Bind(message []byte, target interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Bind", message, target)
	ret0, _ := ret[0].(error)
	return ret0
}

// Bind indicates an expected call of Bind.
func (mr *MockMQTTPublisherSubscriberMockRecorder) Bind(message, target interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Bind", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).Bind), message, target)
}

// Disconnect mocks base method.
func (m *MockMQTTPublisherSubscriber) Disconnect(waitTime uint) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Disconnect", waitTime)
}

// Disconnect indicates an expected call of Disconnect.
func (mr *MockMQTTPublisherSubscriberMockRecorder) Disconnect(waitTime interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Disconnect", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).Disconnect), waitTime)
}

// HealthCheck mocks base method.
func (m *MockMQTTPublisherSubscriber) HealthCheck() types.Health {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck")
	ret0, _ := ret[0].(types.Health)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockMQTTPublisherSubscriberMockRecorder) HealthCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).HealthCheck))
}

// IsSet mocks base method.
func (m *MockMQTTPublisherSubscriber) IsSet() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSet")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsSet indicates an expected call of IsSet.
func (mr *MockMQTTPublisherSubscriberMockRecorder) IsSet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSet", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).IsSet))
}

// Ping mocks base method.
func (m *MockMQTTPublisherSubscriber) Ping() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping")
	ret0, _ := ret[0].(error)
	return ret0
}

// Ping indicates an expected call of Ping.
func (mr *MockMQTTPublisherSubscriberMockRecorder) Ping() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).Ping))
}

// Publish mocks base method.
func (m *MockMQTTPublisherSubscriber) Publish(payload []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", payload)
	ret0, _ := ret[0].(error)
	return ret0
}

// Publish indicates an expected call of Publish.
func (mr *MockMQTTPublisherSubscriberMockRecorder) Publish(payload interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).Publish), payload)
}

// SubscribeToBroker mocks base method.
func (m *MockMQTTPublisherSubscriber) SubscribeToBroker(subscribeFunc SubscribeFunc) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeToBroker", subscribeFunc)
	ret0, _ := ret[0].(error)
	return ret0
}

// SubscribeToBroker indicates an expected call of SubscribeToBroker.
func (mr *MockMQTTPublisherSubscriberMockRecorder) SubscribeToBroker(subscribeFunc interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeToBroker", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).SubscribeToBroker), subscribeFunc)
}

// Unsubscribe mocks base method.
func (m *MockMQTTPublisherSubscriber) Unsubscribe() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unsubscribe")
	ret0, _ := ret[0].(error)
	return ret0
}

// Unsubscribe indicates an expected call of Unsubscribe.
func (mr *MockMQTTPublisherSubscriberMockRecorder) Unsubscribe() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unsubscribe", reflect.TypeOf((*MockMQTTPublisherSubscriber)(nil).Unsubscribe))
}

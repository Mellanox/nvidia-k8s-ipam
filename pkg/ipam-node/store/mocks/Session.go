// Code generated by mockery v2.27.1. DO NOT EDIT.

package mocks

import (
	net "net"

	mock "github.com/stretchr/testify/mock"

	types "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

// Session is an autogenerated mock type for the Session type
type Session struct {
	mock.Mock
}

type Session_Expecter struct {
	mock *mock.Mock
}

func (_m *Session) EXPECT() *Session_Expecter {
	return &Session_Expecter{mock: &_m.Mock}
}

// Cancel provides a mock function with given fields:
func (_m *Session) Cancel() {
	_m.Called()
}

// Session_Cancel_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Cancel'
type Session_Cancel_Call struct {
	*mock.Call
}

// Cancel is a helper method to define mock.On call
func (_e *Session_Expecter) Cancel() *Session_Cancel_Call {
	return &Session_Cancel_Call{Call: _e.mock.On("Cancel")}
}

func (_c *Session_Cancel_Call) Run(run func()) *Session_Cancel_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Session_Cancel_Call) Return() *Session_Cancel_Call {
	_c.Call.Return()
	return _c
}

func (_c *Session_Cancel_Call) RunAndReturn(run func()) *Session_Cancel_Call {
	_c.Call.Return(run)
	return _c
}

// Commit provides a mock function with given fields:
func (_m *Session) Commit() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Session_Commit_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Commit'
type Session_Commit_Call struct {
	*mock.Call
}

// Commit is a helper method to define mock.On call
func (_e *Session_Expecter) Commit() *Session_Commit_Call {
	return &Session_Commit_Call{Call: _e.mock.On("Commit")}
}

func (_c *Session_Commit_Call) Run(run func()) *Session_Commit_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Session_Commit_Call) Return(_a0 error) *Session_Commit_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Session_Commit_Call) RunAndReturn(run func() error) *Session_Commit_Call {
	_c.Call.Return(run)
	return _c
}

// GetLastReservedIP provides a mock function with given fields: pool
func (_m *Session) GetLastReservedIP(pool string) net.IP {
	ret := _m.Called(pool)

	var r0 net.IP
	if rf, ok := ret.Get(0).(func(string) net.IP); ok {
		r0 = rf(pool)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(net.IP)
		}
	}

	return r0
}

// Session_GetLastReservedIP_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetLastReservedIP'
type Session_GetLastReservedIP_Call struct {
	*mock.Call
}

// GetLastReservedIP is a helper method to define mock.On call
//   - pool string
func (_e *Session_Expecter) GetLastReservedIP(pool interface{}) *Session_GetLastReservedIP_Call {
	return &Session_GetLastReservedIP_Call{Call: _e.mock.On("GetLastReservedIP", pool)}
}

func (_c *Session_GetLastReservedIP_Call) Run(run func(pool string)) *Session_GetLastReservedIP_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Session_GetLastReservedIP_Call) Return(_a0 net.IP) *Session_GetLastReservedIP_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Session_GetLastReservedIP_Call) RunAndReturn(run func(string) net.IP) *Session_GetLastReservedIP_Call {
	_c.Call.Return(run)
	return _c
}

// GetReservationByID provides a mock function with given fields: pool, id, ifName
func (_m *Session) GetReservationByID(pool string, id string, ifName string) *types.Reservation {
	ret := _m.Called(pool, id, ifName)

	var r0 *types.Reservation
	if rf, ok := ret.Get(0).(func(string, string, string) *types.Reservation); ok {
		r0 = rf(pool, id, ifName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Reservation)
		}
	}

	return r0
}

// Session_GetReservationByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetReservationByID'
type Session_GetReservationByID_Call struct {
	*mock.Call
}

// GetReservationByID is a helper method to define mock.On call
//   - pool string
//   - id string
//   - ifName string
func (_e *Session_Expecter) GetReservationByID(pool interface{}, id interface{}, ifName interface{}) *Session_GetReservationByID_Call {
	return &Session_GetReservationByID_Call{Call: _e.mock.On("GetReservationByID", pool, id, ifName)}
}

func (_c *Session_GetReservationByID_Call) Run(run func(pool string, id string, ifName string)) *Session_GetReservationByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *Session_GetReservationByID_Call) Return(_a0 *types.Reservation) *Session_GetReservationByID_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Session_GetReservationByID_Call) RunAndReturn(run func(string, string, string) *types.Reservation) *Session_GetReservationByID_Call {
	_c.Call.Return(run)
	return _c
}

// ListPools provides a mock function with given fields:
func (_m *Session) ListPools() []string {
	ret := _m.Called()

	var r0 []string
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}

// Session_ListPools_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListPools'
type Session_ListPools_Call struct {
	*mock.Call
}

// ListPools is a helper method to define mock.On call
func (_e *Session_Expecter) ListPools() *Session_ListPools_Call {
	return &Session_ListPools_Call{Call: _e.mock.On("ListPools")}
}

func (_c *Session_ListPools_Call) Run(run func()) *Session_ListPools_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Session_ListPools_Call) Return(_a0 []string) *Session_ListPools_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Session_ListPools_Call) RunAndReturn(run func() []string) *Session_ListPools_Call {
	_c.Call.Return(run)
	return _c
}

// ListReservations provides a mock function with given fields: pool
func (_m *Session) ListReservations(pool string) []types.Reservation {
	ret := _m.Called(pool)

	var r0 []types.Reservation
	if rf, ok := ret.Get(0).(func(string) []types.Reservation); ok {
		r0 = rf(pool)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Reservation)
		}
	}

	return r0
}

// Session_ListReservations_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListReservations'
type Session_ListReservations_Call struct {
	*mock.Call
}

// ListReservations is a helper method to define mock.On call
//   - pool string
func (_e *Session_Expecter) ListReservations(pool interface{}) *Session_ListReservations_Call {
	return &Session_ListReservations_Call{Call: _e.mock.On("ListReservations", pool)}
}

func (_c *Session_ListReservations_Call) Run(run func(pool string)) *Session_ListReservations_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Session_ListReservations_Call) Return(_a0 []types.Reservation) *Session_ListReservations_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Session_ListReservations_Call) RunAndReturn(run func(string) []types.Reservation) *Session_ListReservations_Call {
	_c.Call.Return(run)
	return _c
}

// ReleaseReservationByID provides a mock function with given fields: pool, id, ifName
func (_m *Session) ReleaseReservationByID(pool string, id string, ifName string) {
	_m.Called(pool, id, ifName)
}

// Session_ReleaseReservationByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ReleaseReservationByID'
type Session_ReleaseReservationByID_Call struct {
	*mock.Call
}

// ReleaseReservationByID is a helper method to define mock.On call
//   - pool string
//   - id string
//   - ifName string
func (_e *Session_Expecter) ReleaseReservationByID(pool interface{}, id interface{}, ifName interface{}) *Session_ReleaseReservationByID_Call {
	return &Session_ReleaseReservationByID_Call{Call: _e.mock.On("ReleaseReservationByID", pool, id, ifName)}
}

func (_c *Session_ReleaseReservationByID_Call) Run(run func(pool string, id string, ifName string)) *Session_ReleaseReservationByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *Session_ReleaseReservationByID_Call) Return() *Session_ReleaseReservationByID_Call {
	_c.Call.Return()
	return _c
}

func (_c *Session_ReleaseReservationByID_Call) RunAndReturn(run func(string, string, string)) *Session_ReleaseReservationByID_Call {
	_c.Call.Return(run)
	return _c
}

// RemovePool provides a mock function with given fields: pool
func (_m *Session) RemovePool(pool string) {
	_m.Called(pool)
}

// Session_RemovePool_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RemovePool'
type Session_RemovePool_Call struct {
	*mock.Call
}

// RemovePool is a helper method to define mock.On call
//   - pool string
func (_e *Session_Expecter) RemovePool(pool interface{}) *Session_RemovePool_Call {
	return &Session_RemovePool_Call{Call: _e.mock.On("RemovePool", pool)}
}

func (_c *Session_RemovePool_Call) Run(run func(pool string)) *Session_RemovePool_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Session_RemovePool_Call) Return() *Session_RemovePool_Call {
	_c.Call.Return()
	return _c
}

func (_c *Session_RemovePool_Call) RunAndReturn(run func(string)) *Session_RemovePool_Call {
	_c.Call.Return(run)
	return _c
}

// Reserve provides a mock function with given fields: pool, id, ifName, meta, address
func (_m *Session) Reserve(pool string, id string, ifName string, meta types.ReservationMetadata, address net.IP) error {
	ret := _m.Called(pool, id, ifName, meta, address)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, types.ReservationMetadata, net.IP) error); ok {
		r0 = rf(pool, id, ifName, meta, address)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Session_Reserve_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Reserve'
type Session_Reserve_Call struct {
	*mock.Call
}

// Reserve is a helper method to define mock.On call
//   - pool string
//   - id string
//   - ifName string
//   - meta types.ReservationMetadata
//   - address net.IP
func (_e *Session_Expecter) Reserve(pool interface{}, id interface{}, ifName interface{}, meta interface{}, address interface{}) *Session_Reserve_Call {
	return &Session_Reserve_Call{Call: _e.mock.On("Reserve", pool, id, ifName, meta, address)}
}

func (_c *Session_Reserve_Call) Run(run func(pool string, id string, ifName string, meta types.ReservationMetadata, address net.IP)) *Session_Reserve_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string), args[2].(string), args[3].(types.ReservationMetadata), args[4].(net.IP))
	})
	return _c
}

func (_c *Session_Reserve_Call) Return(_a0 error) *Session_Reserve_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Session_Reserve_Call) RunAndReturn(run func(string, string, string, types.ReservationMetadata, net.IP) error) *Session_Reserve_Call {
	_c.Call.Return(run)
	return _c
}

// SetLastReservedIP provides a mock function with given fields: pool, ip
func (_m *Session) SetLastReservedIP(pool string, ip net.IP) {
	_m.Called(pool, ip)
}

// Session_SetLastReservedIP_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetLastReservedIP'
type Session_SetLastReservedIP_Call struct {
	*mock.Call
}

// SetLastReservedIP is a helper method to define mock.On call
//   - pool string
//   - ip net.IP
func (_e *Session_Expecter) SetLastReservedIP(pool interface{}, ip interface{}) *Session_SetLastReservedIP_Call {
	return &Session_SetLastReservedIP_Call{Call: _e.mock.On("SetLastReservedIP", pool, ip)}
}

func (_c *Session_SetLastReservedIP_Call) Run(run func(pool string, ip net.IP)) *Session_SetLastReservedIP_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(net.IP))
	})
	return _c
}

func (_c *Session_SetLastReservedIP_Call) Return() *Session_SetLastReservedIP_Call {
	_c.Call.Return()
	return _c
}

func (_c *Session_SetLastReservedIP_Call) RunAndReturn(run func(string, net.IP)) *Session_SetLastReservedIP_Call {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewSession interface {
	mock.TestingT
	Cleanup(func())
}

// NewSession creates a new instance of Session. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewSession(t mockConstructorTestingTNewSession) *Session {
	mock := &Session{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

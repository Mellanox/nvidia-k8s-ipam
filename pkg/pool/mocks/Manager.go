// Code generated by mockery v2.27.1. DO NOT EDIT.

package mocks

import (
	pool "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	mock "github.com/stretchr/testify/mock"

	v1 "k8s.io/api/core/v1"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

type Manager_Expecter struct {
	mock *mock.Mock
}

func (_m *Manager) EXPECT() *Manager_Expecter {
	return &Manager_Expecter{mock: &_m.Mock}
}

// GetPoolByName provides a mock function with given fields: name
func (_m *Manager) GetPoolByName(name string) *pool.IPPool {
	ret := _m.Called(name)

	var r0 *pool.IPPool
	if rf, ok := ret.Get(0).(func(string) *pool.IPPool); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pool.IPPool)
		}
	}

	return r0
}

// Manager_GetPoolByName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetPoolByName'
type Manager_GetPoolByName_Call struct {
	*mock.Call
}

// GetPoolByName is a helper method to define mock.On call
//   - name string
func (_e *Manager_Expecter) GetPoolByName(name interface{}) *Manager_GetPoolByName_Call {
	return &Manager_GetPoolByName_Call{Call: _e.mock.On("GetPoolByName", name)}
}

func (_c *Manager_GetPoolByName_Call) Run(run func(name string)) *Manager_GetPoolByName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *Manager_GetPoolByName_Call) Return(_a0 *pool.IPPool) *Manager_GetPoolByName_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Manager_GetPoolByName_Call) RunAndReturn(run func(string) *pool.IPPool) *Manager_GetPoolByName_Call {
	_c.Call.Return(run)
	return _c
}

// GetPools provides a mock function with given fields:
func (_m *Manager) GetPools() map[string]*pool.IPPool {
	ret := _m.Called()

	var r0 map[string]*pool.IPPool
	if rf, ok := ret.Get(0).(func() map[string]*pool.IPPool); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]*pool.IPPool)
		}
	}

	return r0
}

// Manager_GetPools_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetPools'
type Manager_GetPools_Call struct {
	*mock.Call
}

// GetPools is a helper method to define mock.On call
func (_e *Manager_Expecter) GetPools() *Manager_GetPools_Call {
	return &Manager_GetPools_Call{Call: _e.mock.On("GetPools")}
}

func (_c *Manager_GetPools_Call) Run(run func()) *Manager_GetPools_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Manager_GetPools_Call) Return(_a0 map[string]*pool.IPPool) *Manager_GetPools_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Manager_GetPools_Call) RunAndReturn(run func() map[string]*pool.IPPool) *Manager_GetPools_Call {
	_c.Call.Return(run)
	return _c
}

// Reset provides a mock function with given fields:
func (_m *Manager) Reset() {
	_m.Called()
}

// Manager_Reset_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Reset'
type Manager_Reset_Call struct {
	*mock.Call
}

// Reset is a helper method to define mock.On call
func (_e *Manager_Expecter) Reset() *Manager_Reset_Call {
	return &Manager_Reset_Call{Call: _e.mock.On("Reset")}
}

func (_c *Manager_Reset_Call) Run(run func()) *Manager_Reset_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Manager_Reset_Call) Return() *Manager_Reset_Call {
	_c.Call.Return()
	return _c
}

func (_c *Manager_Reset_Call) RunAndReturn(run func()) *Manager_Reset_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: node
func (_m *Manager) Update(node *v1.Node) error {
	ret := _m.Called(node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.Node) error); ok {
		r0 = rf(node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Manager_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type Manager_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - node *v1.Node
func (_e *Manager_Expecter) Update(node interface{}) *Manager_Update_Call {
	return &Manager_Update_Call{Call: _e.mock.On("Update", node)}
}

func (_c *Manager_Update_Call) Run(run func(node *v1.Node)) *Manager_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*v1.Node))
	})
	return _c
}

func (_c *Manager_Update_Call) Return(_a0 error) *Manager_Update_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Manager_Update_Call) RunAndReturn(run func(*v1.Node) error) *Manager_Update_Call {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewManager interface {
	mock.TestingT
	Cleanup(func())
}

// NewManager creates a new instance of Manager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewManager(t mockConstructorTestingTNewManager) *Manager {
	mock := &Manager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
